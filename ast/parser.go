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
	stk []Value
	ic  map[string]string
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
	h.push(v)
	return nil
}

func (h *parseHandler) push(v Value) { h.stk = append(h.stk, v) }

func (h *parseHandler) BeginObject(loc jtree.Anchor) error {
	h.push(objectStub{})
	return nil
}

func (h *parseHandler) EndObject(loc jtree.Anchor) error {
	for i := len(h.stk) - 1; i >= 0; i-- {
		if _, ok := h.stk[i].(objectStub); ok {
			o := make(Object, 0, len(h.stk)-i-1)
			for j := i + 1; j < len(h.stk); j++ {
				o = append(o, h.stk[j].(*Member))
			}
			h.stk = h.stk[:i]
			return h.reduceValue(o)
		}
	}
	panic("unbalanced EndObject")
}

func (h *parseHandler) BeginArray(loc jtree.Anchor) error {
	h.push(arrayStub{})
	return nil
}

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
	key := Quoted{text: mem.S(h.intern(loc.Text()))}
	h.push(&Member{Key: key})
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error { return nil }

func (h *parseHandler) Value(loc jtree.Anchor) error {
	switch loc.Token() {
	case jtree.String:
		return h.reduceValue(Quoted{text: mem.B(loc.Copy())})
	case jtree.Integer:
		return h.reduceValue(Number{text: mem.B(loc.Copy()), isInt: true})
	case jtree.Number:
		return h.reduceValue(Number{text: mem.B(loc.Copy()), isInt: false})
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

type arrayStub struct{ Value }

type objectStub struct{ Value }
