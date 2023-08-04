// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

// Package cursor implements traversal over the AST of a JSON value.
package cursor

import (
	"fmt"

	"github.com/creachadair/jtree/ast"
)

// Path traverses a sequential path into the structure of v where path elements
// are as documented for the Cursor.Down method.  This is a convenience wrapper
// for creating a cursor, applying path, and retrieving its value.
func Path[T ast.Value](v ast.Value, path ...any) (T, error) {
	c := New(v).Down(path...)
	var result T
	if err := c.Err(); err != nil {
		return result, err
	}
	v, ok := c.Value().(T)
	if !ok {
		return result, fmt.Errorf("wrong value type %T", c.Value())
	}
	return v.(T), nil
}

// A Cursor is a pointer that navigates into the structure of a ast.Value.
type Cursor struct {
	org ast.Value
	stk []ast.Value
	err error
}

// New constructs a new Cursor to traverse the structure of origin.
func New(origin ast.Value) *Cursor { return &Cursor{org: origin} }

// Origin returns the origin value of c.
func (c *Cursor) Origin() ast.Value { return c.org }

// AtOrigin reports whether c is at its origin.
func (c *Cursor) AtOrigin() bool { return len(c.stk) == 0 }

// Value reports the current value under the cursor.
func (c *Cursor) Value() ast.Value {
	if c.AtOrigin() {
		return c.org
	}
	return c.stk[len(c.stk)-1]
}

// Path reports the complete sequence of values from the origin to the current
// location in c.
func (c *Cursor) Path() []ast.Value {
	return append([]ast.Value{c.org}, c.stk...)
}

// Err reports the error from the most recent traversal operation, if any.
func (c *Cursor) Err() error { return c.err }

// Up moves the cursor one position upward in the structure, if possible.
// It returns c to permit chaining.
func (c *Cursor) Up() *Cursor {
	if n := len(c.stk); n > 0 {
		c.stk = c.stk[:n-1]
	}
	return c
}

// Reset resets the cursor to its origin and clears its error.
func (c *Cursor) Reset() { c.stk = c.stk[:0]; c.err = nil }

// Down traverses a sequential path into the structure of c starting from the
// current value, where path elements are either strings (denoting object
// keys), integers (denoting offsets into arrays), functions (see below), or
// nil.  If the path is valid, the element reached is returned. If the path
// cannot be completely consumed, traversal stops and an error is recorded. Use
// Err to recover the error.
//
// If a path element is a string, the corresponding value must be an object,
// and the string resolves an object member with that name. If this is the last
// element of the path, the member is returned; otherwise, subsequent path
// elements continue from the value of that member. Use a nil path element to
// resolve an object member at the end of a path.
//
// If a path element is an integer, the corresponding value must be an array or
// object, and the integer resolves to an index in the array or object.
// Negative indices count backward from the end (-1 is last, -2 second last).
// An error is reported if the index is out of bounds.
//
// If a path element is a function, the function is executed and its result
// becomes the next object in the sequence. The function must have a signature
//
//	func(ast.Value) (ast.Value, error)
//
// If the function reports an error, traversal stops and the error is recorded.
func (c *Cursor) Down(path ...any) *Cursor {
	c.err = nil // reset error
	cur := c.Value()
	for _, elt := range path {
		// If the previous step ended on an object member, interpret the next
		// path element relative to the value of that member.
		if m, ok := cur.(*ast.Member); ok {
			cur = c.push(m.Value)
		}

		switch t := elt.(type) {
		case string:
			switch e := cur.(type) {
			case ast.Object:
				m := e.Find(t)
				if m == nil {
					return c.setErrorf("key %q not found", t)
				}
				cur = c.push(m)
			default:
				return c.setErrorf("cannot traverse %T with %q", cur, elt)
			}

		case int:
			switch e := cur.(type) {
			case ast.Array:
				i, ok := fixArrayBound(len(e), t)
				if !ok {
					return c.setErrorf("array index %d out of bounds (n=%d)", i, len(e))
				}
				cur = c.push(e[i])
			case ast.Object:
				i, ok := fixArrayBound(len(e), t)
				if !ok {
					return c.setErrorf("object index %d out of bounds (n=%d)", i, len(e))
				}
				cur = c.push(e[i])
			default:
				return c.setErrorf("cannot traverse %T with %v", cur, elt)
			}

		case func(ast.Value) (ast.Value, error):
			next, err := t(cur)
			if err != nil {
				c.err = err
				return c
			}
			cur = c.push(next)

		case nil:
			// Do nothing. This case supports indirecting through a member at the
			// end of the path.

		default:
			return c.setErrorf("invalid path element %T", elt)
		}
	}
	return c
}

func (c *Cursor) push(v ast.Value) ast.Value { c.stk = append(c.stk, v); return v }

func (c *Cursor) setErrorf(msg string, args ...any) *Cursor {
	c.err = fmt.Errorf(msg, args...)
	return c
}

func fixArrayBound(n, i int) (int, bool) {
	if i < 0 {
		i += n
	}
	return i, i >= 0 && i < n
}
