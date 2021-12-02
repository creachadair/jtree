package ast

import (
	"errors"
	"fmt"
	"io"
	"strconv"

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
	stk []Value
}

func (h *parseHandler) reduce() error {
	if len(h.stk) > 1 {
		switch prev := h.stk[len(h.stk)-2].(type) {
		case *Member:
			prev.Value = h.pop()
			prev.end = prev.Value.Span().End
		case *Object:
			h.pop() // already in the object
		case *Array:
			prev.Values = append(prev.Values, h.pop())
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
	h.push(&Object{pos: loc.Location().Pos})
	return nil
}

func (h *parseHandler) EndObject(loc jtree.Anchor) error {
	h.top().(*Object).end = loc.Location().End
	return h.reduce()
}

func (h *parseHandler) BeginArray(loc jtree.Anchor) error {
	h.push(&Array{pos: loc.Location().Pos})
	return nil
}

func (h *parseHandler) EndArray(loc jtree.Anchor) error {
	h.top().(*Array).end = loc.Location().End
	return h.reduce()
}

func (h *parseHandler) BeginMember(loc jtree.Anchor) error {
	dec, err := jtree.UnescapeString(loc.Text())
	if err != nil {
		return err
	}

	// The object this member belongs to is atop the stack.  Add a pointer to
	// the new member into its collection eagerly, so that when reducing the
	// stack after the value is known, we don't have to reduce multiple times.

	mem := &Member{pos: loc.Location().Pos, Key: dec}
	obj := h.top().(*Object)
	obj.Members = append(obj.Members, mem)
	h.push(mem)
	return nil
}

func (h *parseHandler) EndMember(loc jtree.Anchor) error { return h.reduce() }

func (h *parseHandler) Value(loc jtree.Anchor) error {
	span := loc.Location()
	d := datum{pos: span.Pos, end: span.End, text: loc.Text()}
	switch loc.Token() {
	case jtree.String:
		dec, err := jtree.UnescapeString(d.text)
		if err != nil {
			return err
		}
		h.push(String{datum: d, Value: dec})
	case jtree.Integer:
		v, err := strconv.ParseInt(d.text, 10, 64)
		if err != nil {
			return err
		}
		h.push(Integer{datum: d, Value: v})
	case jtree.Number:
		v, err := strconv.ParseFloat(d.text, 64)
		if err != nil {
			return err
		}
		h.push(Number{datum: d, Value: v})
	case jtree.True, jtree.False:
		h.push(Bool{datum: d, Value: loc.Token() == jtree.True})
	case jtree.Null:
		h.push(Null{datum: d})
	default:
		return fmt.Errorf("unknown value %v", loc.Token())
	}
	return h.reduce()
}

func (h *parseHandler) SyntaxError(loc jtree.Anchor, err error) error { return err }

func (h *parseHandler) EndOfInput(loc jtree.Anchor) {}
