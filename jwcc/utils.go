// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc

import (
	"fmt"
	"strings"

	"github.com/creachadair/jtree/ast"
)

// Path traverses a sequential path through the structure of a value starting
// at v, where path elements are either strings (denoting object keys) or
// integers (denoting offsets into arrays).  If the path is valid, the element
// reached is returned. In case of error, the input v is returned along with
// the error.
//
// If a path element is a string, the corresponding value must be an object,
// and the string resolves an object member with that name.
//
// If a path element is an integer, the corresponding value must be an array,
// and the integer resolves to an index in the array. Negative indices count
// backward from the end of the array (-1 is last, -2 second last, etc.).
//
// If a path element is a function, the function is executed and its result
// becomes the next object in the sequence. The function must have a signature
//
//	func(jwcc.Value) (jwcc.Value, error)
//
// If the function fails, the traversal reports its error.
func Path(v Value, path ...any) (Value, error) {
	cur := v
	for _, elt := range path {
		if m, ok := cur.(*Member); ok {
			cur = m.Value
		}
		switch t := elt.(type) {
		case string:
			switch c := cur.(type) {
			case *Object:
				m := c.Find(t)
				if m == nil {
					return v, fmt.Errorf("key %q not found", t)
				}
				cur = m
			default:
				return v, fmt.Errorf("cannot traverse %T with %q", cur, elt)
			}
		case int:
			switch c := cur.(type) {
			case *Array:
				i, ok := fixArrayBound(len(c.Values), t)
				if !ok {
					return v, fmt.Errorf("array index %d out of bounds (n=%d)", i, len(c.Values))
				}
				cur = c.Values[i]
			default:
				return v, fmt.Errorf("cannot traverse %T with %v", cur, elt)
			}
		case func(Value) (Value, error):
			next, err := t(cur)
			if err != nil {
				return v, err
			}
			cur = next
		default:
			return nil, fmt.Errorf("invalid path element %T", elt)
		}
	}
	return cur, nil
}

func fixArrayBound(n, i int) (int, bool) {
	if i < 0 {
		i += n
	}
	return i, i >= 0 && i < n
}

// CleanComments combines and removes comment markers from the given comments,
// returning a slice of plain lines of text. Leading and trailing spaces are
// removed from the lines.
func CleanComments(coms ...string) []string {
	var out []string
	for _, com := range coms {
		_, text := classifyComment(com)
		lines := strings.Split(text, "\n")
		outdentCommentLines(lines)
		for _, line := range lines {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

// Decorate converts an ast.Value into an equivalent jwcc.Value.
func Decorate(v ast.Value) Value {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case ast.Object:
		o := &Object{Members: make([]*Member, len(t))}
		for i, m := range t {
			o.Members[i] = &Member{Key: m.Key, Value: Decorate(m.Value)}
		}
		return o
	case ast.Array:
		a := &Array{Values: make([]Value, len(t))}
		for i, v := range t {
			a.Values[i] = Decorate(v)
		}
		return a
	default:
		return &Datum{Value: v}
	}
}
