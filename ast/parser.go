// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast

import (
	"errors"
	"fmt"
	"io"

	"github.com/creachadair/jtree"
	"go4.org/mem"
)

// A Parser parses and returns JSON values from a reader.
type Parser struct {
	h  *parseHandler
	st *jtree.Stream
}

// AllowJWCC configures p to accept (true) or reject (false) JWCC extensions
// supporting comments and trailing commas.
//
// See: https://nigeltao.github.io/blog/2021/json-with-commas-comments.html
func (p *Parser) AllowJWCC(ok bool) {
	p.st.AllowComments(ok)
	p.st.AllowTrailingCommas(ok)
}

// NewParser constructs a parser that consumes input from r.
func NewParser(r io.Reader) *Parser {
	h := &parseHandler{ic: make(map[string]string)}
	return &Parser{h: h, st: jtree.NewStream(r)}
}

// Parse parses and returns the next JSON value from its input.
// It returns io.EOF if no further values are available.
func (p *Parser) Parse() (Value, error) {
	if err := p.st.ParseOne(p.h); err != nil {
		return nil, err
	} else if len(p.h.stk) != 1 {
		return nil, errors.New("incomplete value")
	}
	out := p.h.stk[0]
	p.h.stk = p.h.stk[:0]
	return out, nil
}

// Parse parses and returns the JSON values from r. In case of error, any
// complete values already parsed are returned along with the error.
// It reports ErrEmptyInput if the input is entirely empty.
func Parse(r io.Reader) ([]Value, error) {
	p := NewParser(r)
	var vs []Value
	for {
		v, err := p.Parse()
		if err == io.EOF {
			if len(vs) == 0 {
				return nil, ErrEmptyInput
			}
			return vs, nil
		} else if err != nil {
			return vs, err
		}
		vs = append(vs, v)
	}
}

// ParseSingle parses and returns a single JSON value from r. If r contains
// more data after the first value, ParseOne returns the first value along with
// an ErrExtraInput error.
func ParseSingle(r io.Reader) (Value, error) {
	p := NewParser(r)
	v, err := p.Parse()
	if err == io.EOF {
		return v, ErrEmptyInput
	} else if err != nil {
		return nil, err
	}

	// Trigger the parser with a handler that fails on any non-empty input.
	if err := p.st.ParseOne(noMoreInput{}); err != io.EOF {
		return v, err
	}
	return v, nil
}

// A parseHandler implements the jtree.Handler interface to construct abstract
// syntax trees for JSON values.
type parseHandler struct {
	stk  []Value
	tbuf [][]byte
	ic   map[string]string
}

// intern returns an interned copy of text.
func (h *parseHandler) intern(text []byte) string {
	s, ok := h.ic[string(text)]
	if ok {
		return s
	}
	s = string(text)
	h.ic[s] = s
	return s
}

// copyOf returns a copy of text.  Small allocations are batched to reduce overhead.
func (h *parseHandler) copyOf(text []byte) mem.RO {
	const minBlockSlop = 4
	const smallSizeFraction = 8
	const bufBlockBytes = 16384

	// For values bigger than smallSizeFraction of the block size, don't bother
	// batching, make an outright copy.
	if len(text) >= bufBlockBytes/smallSizeFraction {
		return mem.B(append([]byte(nil), text...))
	}

	// Look for a block with space enough to hold a copy of text.
	i := 0
	for i < len(h.tbuf) {
		if n := len(h.tbuf[i]) + len(text); n < cap(h.tbuf[i]) {
			// There is room in this block.
			break
		} else if cap(h.tbuf[i])-len(text) < minBlockSlop {
			// There is no room in this block, but it is nearly-enough full.
			// Allocate a fresh block at this location and release the old one.
			// The old block will be retained until all its tokens are released.
			h.tbuf[i] = make([]byte, 0, bufBlockBytes)
			break
		}
		i++
	}
	if i == len(h.tbuf) {
		// No block had room; add a new empty one to the arena.
		h.tbuf = append(h.tbuf, make([]byte, 0, bufBlockBytes))
	}
	s := len(h.tbuf[i])
	h.tbuf[i] = append(h.tbuf[i], text...)
	return mem.B(h.tbuf[i][s : s+len(text)])
}

func (h *parseHandler) mergeTop(v Value) {
	old := &h.stk[len(h.stk)-1]
	switch t := (*old).(type) {
	case *Member:
		t.Value = v
	case Object:
		// already in the object
	case Array:
		*old = append(t, v)
	default:
		h.push(v)
	}
}

func (h *parseHandler) reduce() error {
	if len(h.stk) > 1 {
		h.mergeTop(h.pop())
	}
	return nil
}

func (h *parseHandler) reduceValue(v Value) error {
	if len(h.stk) > 0 {
		h.mergeTop(v)
	} else {
		h.push(v)
	}
	return nil
}

func (h *parseHandler) pop() Value {
	last := h.stk[len(h.stk)-1]
	h.stk = h.stk[:len(h.stk)-1]
	return last
}

func (h *parseHandler) push(v Value) { h.stk = append(h.stk, v) }

func (h *parseHandler) BeginObject(loc jtree.Anchor) error {
	h.push(Object(nil))
	return nil
}

func (h *parseHandler) EndObject(loc jtree.Anchor) error {
	return h.reduce()
}

func (h *parseHandler) BeginArray(loc jtree.Anchor) error {
	h.push(Array(nil))
	return nil
}

func (h *parseHandler) EndArray(loc jtree.Anchor) error {
	return h.reduce()
}

func (h *parseHandler) BeginMember(loc jtree.Anchor) error {
	// The object this member belongs to is atop the stack.  Add a pointer to
	// the new member into its collection eagerly, so that when reducing the
	// stack after the value is known, we don't have to reduce multiple times.

	mem := &Member{Key: Quoted{text: mem.S(h.intern(loc.Text()))}}
	top := &h.stk[len(h.stk)-1]
	obj := (*top).(Object)
	*top = append(obj, mem)
	h.push(mem)
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error { return h.reduce() }

func (h *parseHandler) Value(loc jtree.Anchor) error {
	switch loc.Token() {
	case jtree.String:
		return h.reduceValue(Quoted{text: h.copyOf(loc.Text())})
	case jtree.Integer:
		return h.reduceValue(Number{text: h.copyOf(loc.Text()), isInt: true})
	case jtree.Number:
		return h.reduceValue(Number{text: h.copyOf(loc.Text()), isInt: false})
	case jtree.True, jtree.False:
		return h.reduceValue(Bool(loc.Token() == jtree.True))
	case jtree.Null:
		return h.reduceValue(Null)
	default:
		return fmt.Errorf("unknown value %v", loc.Token())
	}
}

func (h *parseHandler) SyntaxError(loc jtree.Anchor, err error) error { return err }

func (h *parseHandler) EndOfInput(loc jtree.Anchor) {}

// ErrEmptyInput is a sentinel error reported by Parse if the input is empty.
var ErrEmptyInput = errors.New("empty input")

// ErrExtraInput is a sentinel error reported by ParseOne if the input contains
// additional values after the first one.
var ErrExtraInput = errors.New("extra data after value")

// A noMoreInput is a jtree.Handler that reports an error for any input.
type noMoreInput struct{}

func (noMoreInput) BeginObject(jtree.Anchor) error { return ErrExtraInput }
func (noMoreInput) EndObject(jtree.Anchor) error   { return ErrExtraInput }
func (noMoreInput) BeginArray(jtree.Anchor) error  { return ErrExtraInput }
func (noMoreInput) EndArray(jtree.Anchor) error    { return ErrExtraInput }
func (noMoreInput) BeginMember(jtree.Anchor) error { return ErrExtraInput }
func (noMoreInput) EndMember(jtree.Anchor) error   { return ErrExtraInput }
func (noMoreInput) Value(jtree.Anchor) error       { return ErrExtraInput }
func (noMoreInput) EndOfInput(jtree.Anchor)        {}
