// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast

import (
	"errors"
	"fmt"
	"io"

	"github.com/creachadair/jtree"
)

// A Parser parses and returns JSON values from a reader.
type Parser struct {
	h  *parseHandler
	st *jtree.Stream
}

// NewParser constructs a parser that consumes input from r.
func NewParser(r io.Reader) *Parser {
	return &Parser{h: new(parseHandler), st: jtree.NewStream(r)}
}

// Parse parses and returns the next JSON value from its input.
// It returns io.EOF if no further values are available.
func (p *Parser) Parse() (Value, error) {
	if err := p.st.ParseOne(p.h); err != nil {
		return nil, err
	} else if len(p.h.stk) != 1 {
		return nil, errors.New("incomplete value")
	}
	out := *p.h.stk[0]
	p.h.stk = p.h.stk[:0]
	return out, nil
}

// Parse parses and returns the JSON values from r. In case of error, any
// complete values already parsed are returned along with the error.
func Parse(r io.Reader) ([]Value, error) {
	p := NewParser(r)
	var vs []Value
	for {
		v, err := p.Parse()
		if err == io.EOF {
			return vs, nil
		} else if err != nil {
			return vs, err
		}
		vs = append(vs, v)
	}
}

// A parseHandler implements the jtree.Handler interface to construct abstract
// syntax trees for JSON values.
type parseHandler struct {
	stk  []*Value
	tbuf [][]byte
}

// copyOf returns a copy of text.  Allocations are batched to reduce overhead.
func (h *parseHandler) copyOf(text []byte) []byte {
	const minBlockSlop = 16
	const bufBlockBytes = 16384

	if len(text) >= bufBlockBytes {
		return append([]byte(nil), text...)
	}

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
	return h.tbuf[i][s : s+len(text)]
}

func merge(old *Value, v Value) {
	switch t := (*old).(type) {
	case *Member:
		t.Value = v
	case Object:
		// already in the object
	case Array:
		*old = append(t, v)
	}
}

func (h *parseHandler) reduce() error {
	if len(h.stk) > 1 {
		v := h.pop()
		merge(h.top(), v)
	}
	return nil
}

func (h *parseHandler) reduceValue(v Value) error {
	if len(h.stk) > 0 {
		merge(h.top(), v)
	}
	return nil
}

func (h *parseHandler) top() *Value { return h.stk[len(h.stk)-1] }

func (h *parseHandler) pop() Value {
	last := h.top()
	h.stk = h.stk[:len(h.stk)-1]
	return *last
}

func (h *parseHandler) push(v Value) { h.stk = append(h.stk, &v) }

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

	mem := &Member{Key: String{text: h.copyOf(loc.Text())}}
	top := h.top()
	obj := (*top).(Object)
	*top = append(obj, mem)
	h.push(mem)
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error { return h.reduce() }

func (h *parseHandler) Value(loc jtree.Anchor) error {
	switch loc.Token() {
	case jtree.String:
		return h.reduceValue(&String{text: h.copyOf(loc.Text())})
	case jtree.Integer:
		return h.reduceValue(&Integer{text: h.copyOf(loc.Text())})
	case jtree.Number:
		return h.reduceValue(&Number{text: h.copyOf(loc.Text())})
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
