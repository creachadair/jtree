// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/creachadair/jtree"
	"go4.org/mem"
)

// A Parser parses and returns JSON values from a reader.
type Parser struct {
	h  *parseHandler
	st *jtree.Stream
}

// AllowJWCC configures p to accept (true) or reject (false) JWCC extensions
// supporting comments and trailing commas.  Setting this option to true does
// not include comments in the result; it only instructs the parser to accept
// input that includes comments and trailing commas.
//
// See: https://nigeltao.github.io/blog/2021/json-with-commas-comments.html
func (p *Parser) AllowJWCC(ok bool) {
	p.st.AllowComments(ok)
	p.st.AllowTrailingCommas(ok)
}

// NewParser constructs a parser that consumes input from r.
func NewParser(r io.Reader) *Parser {
	h := &parseHandler{ic: make(jtree.Interner)}
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
// It reports [ErrEmptyInput] if the input is entirely empty.
func Parse(r io.Reader) ([]Value, error) {
	var vs []Value
	for v, err := range ParseRange(r) {
		if err != nil {
			return vs, err
		}
		vs = append(vs, v)
	}
	if len(vs) == 0 {
		return nil, ErrEmptyInput
	}
	return vs, nil
}

// ParseRange parses and yields the JSON values from r.  Each pair produced by
// the iterator is either a valid JSON value and nil error, or a nil value and
// a non-nil parse error.  If and when an error occurs, the iterator stops.
// If the input is empty, the iterator yields no values.
func ParseRange(r io.Reader) iter.Seq2[Value, error] {
	p := NewParser(r)
	return func(yield func(Value, error) bool) {
		for {
			v, err := p.Parse()
			if err == io.EOF {
				return // no more values
			} else if err != nil {
				yield(nil, err)
				return
			}

			if !yield(v, nil) {
				return
			}
		}
	}
}

// ParseSingle parses and returns a single JSON value from r. If r contains no
// values, ParseSingle reports nil, [ErrEmptyInput].
// If r contains more data after the first value, ParseSingle returns the first
// value, and [ErrExtraInput].
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
	stk []Value
	ic  jtree.Interner
}

func (h *parseHandler) reduceValue(v Value) error {
	// If there is an incomplete object member waiting for a value, populate the
	// value directly.
	if n := len(h.stk); n > 0 {
		m, ok := h.stk[n-1].(*Member)
		if ok {
			m.Value = v
			return nil
		}
	}
	// Otherwise, accumulate the value normally.
	return h.push(v)
}

func (h *parseHandler) push(v Value) error { h.stk = append(h.stk, v); return nil }

func (h *parseHandler) BeginObject(loc jtree.Anchor) error { return h.push(objectStub{}) }

func (h *parseHandler) EndObject(loc jtree.Anchor) error {
	for i := len(h.stk) - 1; i >= 0; i-- {
		if _, ok := h.stk[i].(objectStub); ok {
			o := make(Object, 0, len(h.stk)-i-1)
			for _, m := range h.stk[i+1:] {
				o = append(o, m.(*Member))
			}
			h.stk = h.stk[:i]
			return h.reduceValue(o)
		}
	}
	panic("unbalanced EndObject")
}

func (h *parseHandler) BeginArray(loc jtree.Anchor) error { return h.push(arrayStub{}) }

func (h *parseHandler) EndArray(loc jtree.Anchor) error {
	for i := len(h.stk) - 1; i >= 0; i-- {
		if _, ok := h.stk[i].(arrayStub); ok {
			a := make(Array, len(h.stk)-i-1)
			copy(a, h.stk[i+1:])
			h.stk = h.stk[:i]
			return h.reduceValue(a)
		}
	}
	panic("unbalanced EndArray")
}

func (h *parseHandler) BeginMember(loc jtree.Anchor) error {
	return h.push(&Member{Key: Quoted(h.ic.Intern(loc.Text()))})
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error { return nil }

func (h *parseHandler) Value(loc jtree.Anchor) error {
	v, err := AnchorValue(loc)
	if err != nil {
		return err
	}
	return h.reduceValue(v)
}

// AnchorValue constructs a Value from the specified anchor, or reports an
// error if the anchor does not record a value.
func AnchorValue(loc jtree.Anchor) (Value, error) {
	switch loc.Token() {
	case jtree.String:
		return quotedText{data: mem.B(loc.Copy())}, nil
	case jtree.Integer:
		return rawNumber{text: loc.Copy(), isInt: true}, nil
	case jtree.Number:
		return rawNumber{text: loc.Copy(), isInt: false}, nil
	case jtree.True, jtree.False:
		return Bool(loc.Token() == jtree.True), nil
	case jtree.Null:
		return Null, nil
	default:
		return nil, fmt.Errorf("unknown value %v", loc.Token())
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

type arrayStub struct{ Value }

type objectStub struct{ Value }
