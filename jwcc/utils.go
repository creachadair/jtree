// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc

import (
	"fmt"
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
		switch t := elt.(type) {
		case string:
			switch c := cur.(type) {
			case *Object:
				m := c.Find(t)
				if m == nil {
					return v, fmt.Errorf("key %q not found", t)
				}
				cur = m.Value
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
