// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast

import (
	"errors"
	"fmt"
	"io"

	"github.com/creachadair/jtree"
)

// Parse parses and returns the JSON values from r. In case of error, any
// complete values already parsed are returned along with the error.
func Parse(r io.Reader) ([]Value, error) {
	h := new(parseHandler)
	st := jtree.NewStream(r)
	var vs []Value
	for {
		if err := st.ParseOne(h); err == io.EOF {
			return vs, nil
		} else if err != nil {
			return vs, err
		}
		if len(h.stk) != 1 {
			return vs, errors.New("incomplete value")
		}
		vs = append(vs, h.stk[0])
		h.stk = h.stk[:0]
	}
}

// A parseHandler implements the jtree.Handler interface to construct abstract
// syntax trees for JSON values.
type parseHandler struct {
	stk  []Value
	tbuf [][]byte
}

// intern interns a copy of text and returns a slice of the copy.  Allocations
// are batched to reduce allocation overhead.
func (h *parseHandler) intern(text []byte) []byte {
	const bufBlockBytes = 8192

	if len(text) >= bufBlockBytes {
		return append([]byte(nil), text...)
	}

	i := 0
	for i < len(h.tbuf) {
		if len(h.tbuf[i])+len(text) < cap(h.tbuf[i]) {
			break
		}
		i++
	}
	if i == len(h.tbuf) {
		h.tbuf = append(h.tbuf, make([]byte, 0, bufBlockBytes))
	}
	s := len(h.tbuf[i])
	h.tbuf[i] = append(h.tbuf[i], text...)
	return h.tbuf[i][s : s+len(text)]
}

func (h *parseHandler) reduce() error {
	if len(h.stk) > 1 {
		v := h.pop()
		return h.reduceValue(v)
	}
	return nil
}

func (h *parseHandler) reduceValue(v Value) error {
	if len(h.stk) > 0 {
		switch prev := h.stk[len(h.stk)-1].(type) {
		case *Member:
			prev.Value = v
		case *Object:
			// already in the object
		case *Array:
			prev.Values = append(prev.Values, v)
		}
	}
	return nil
}

func (h *parseHandler) top() Value { return h.stk[len(h.stk)-1] }

func (h *parseHandler) pop() Value {
	last := h.top()
	h.stk = h.stk[:len(h.stk)-1]
	return last
}

func (h *parseHandler) push(v Value) { h.stk = append(h.stk, v) }

func (h *parseHandler) BeginObject(loc jtree.Anchor) error {
	h.push(new(Object))
	return nil
}

func (h *parseHandler) EndObject(loc jtree.Anchor) error {
	return h.reduce()
}

func (h *parseHandler) BeginArray(loc jtree.Anchor) error {
	h.push(new(Array))
	return nil
}

func (h *parseHandler) EndArray(loc jtree.Anchor) error {
	return h.reduce()
}

func (h *parseHandler) BeginMember(loc jtree.Anchor) error {
	// The object this member belongs to is atop the stack.  Add a pointer to
	// the new member into its collection eagerly, so that when reducing the
	// stack after the value is known, we don't have to reduce multiple times.

	mem := &Member{key: h.intern(loc.Text())}
	obj := h.top().(*Object)
	obj.Members = append(obj.Members, mem)
	h.push(mem)
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error { return h.reduce() }

func (h *parseHandler) Value(loc jtree.Anchor) error {
	d := datum{text: h.intern(loc.Text())}
	switch loc.Token() {
	case jtree.String:
		return h.reduceValue(&String{datum: d})
	case jtree.Integer:
		return h.reduceValue(&Integer{datum: d})
	case jtree.Number:
		return h.reduceValue(&Number{datum: d})
	case jtree.True, jtree.False:
		ok := loc.Token() == jtree.True
		return h.reduceValue(&Bool{datum: d, value: ok})
	case jtree.Null:
		return h.reduceValue(&Null{datum: d})
	default:
		return fmt.Errorf("unknown value %v", loc.Token())
	}
}

func (h *parseHandler) SyntaxError(loc jtree.Anchor, err error) error { return err }

func (h *parseHandler) EndOfInput(loc jtree.Anchor) {}
