// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

// Package ast defines an abstract syntax tree for JSON values,
// and a parser that constructs syntax trees from JSON source.
package ast

import (
	"strconv"
	"strings"

	"github.com/creachadair/jtree/internal/escape"
	"go4.org/mem"
)

// A Value is an arbitrary JSON value.
type Value interface {
	// JSON converts the value into JSON source text.
	JSON() string
}

// A Stringer is a Value that can be rendered as a string.
type Stringer interface {
	Value
	String() string
}

// An Object is a collection of key-value members.
type Object []*Member

// Find returns the first member of o with the given key, or nil.
func (o Object) Find(key string) *Member {
	for _, m := range o {
		if m.Key.String() == key {
			return m
		}
	}
	return nil
}

// Len returns the number of members in the object.
func (o Object) Len() int { return len(o) }

// JSON renders o as JSON text.
func (o Object) JSON() string {
	if len(o) == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteByte('{')
	sb.WriteString(o[0].JSON())
	for _, elt := range o[1:] {
		sb.WriteByte(',')
		sb.WriteString(elt.JSON())
	}
	sb.WriteByte('}')
	return sb.String()
}

// A Member is a single key-value pair belonging to an Object. A Key must be a
// value convertible to a string, typically either an ast.Quoted or ast.String.
type Member struct {
	Key   Stringer
	Value Value
}

// Field constructs an object member with the given key and value.
func Field(key string, val Value) *Member {
	return &Member{Key: String(key), Value: val}
}

// JSON renders the member as JSON text.
func (m *Member) JSON() string {
	k, v := m.Key.JSON(), m.Value.JSON()
	buf := make([]byte, len(k)+len(v)+1)
	n := copy(buf, k)
	buf[n] = ':'
	copy(buf[n+1:], v)
	return string(buf)
}

// An Array is a sequence of values.
type Array []Value

// Len returns the number of elements in a.
func (a Array) Len() int { return len(a) }

// JSON renders the array as JSON text.
func (a Array) JSON() string {
	if len(a) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(a[0].JSON())
	for _, elt := range a[1:] {
		sb.WriteByte(',')
		sb.WriteString(elt.JSON())
	}
	sb.WriteByte(']')
	return sb.String()
}

// A Number is a numeric literal.
type Number struct{ text []byte }

// JSON renders n as JSON text.
func (n Number) JSON() string { return string(n.text) }

// Float returns a representation of n as a Float. It panics if n is not
// representable as a floating-point value.
func (n Number) Float() Float {
	v, err := mem.ParseFloat(mem.B(n.text), 64)
	if err != nil {
		panic(err)
	}
	return Float(v)
}

// Int returns a representation of n as an Int.  If n is valid but has
// fractional parts, the fractions are truncated; otherwise Int panics if n is
// not representable as a number.
func (n Number) Int() Int {
	v, err := mem.ParseFloat(mem.B(n.text), 64)
	if err != nil {
		panic(err)
	}
	return Int(v)
}

// A Float is represents a floating-point number.
type Float float64

// JSON renders f as JSON text.
func (f Float) JSON() string { return strconv.FormatFloat(float64(f), 'g', -1, 64) }

// Value returns f as a float64. It is shorthand for a type conversion.
func (f Float) Value() float64 { return float64(f) }

// An Int represents an integer number.
type Int int64

// JSON renders z as JSON text.
func (z Int) JSON() string { return strconv.FormatInt(int64(z), 10) }

// Value returns z as an int64. It is shorthand for a type conversion.
func (z Int) Value() int64 { return int64(z) }

// A Bool is a Boolean constant, true or false.
type Bool bool

// Value reports the truth value of the Boolean.
func (b Bool) Value() bool { return bool(b) }

// JSON returns b as JSON text.
func (b Bool) JSON() string {
	if b {
		return "true"
	}
	return "false"
}

// A Quoted is a quoted string value.
type Quoted struct{ text []byte }

// Unquote returns the unescaped text of the string.
func (q Quoted) Unquote() String {
	if len(q.text) == 0 {
		return ""
	}
	dec, err := escape.Unquote(mem.B(q.text[1 : len(q.text)-1]))
	if err != nil {
		panic(err)
	}
	return String(dec)
}

// String returns the unescaped text of the string.
func (q Quoted) String() string { return string(q.Unquote()) }

// Len returns the length in bytes of the unquoted text of q.
func (q Quoted) Len() int { return q.Unquote().Len() }

// JSON returns the JSON encoding of q.
func (q Quoted) JSON() string { return string(q.text) }

// A String is an unquoted string value.
type String string

// Len returns the length in bytes of s.
func (s String) Len() int { return len(s) }

// Quote converts s into its quoted representation.
func (s String) Quote() Quoted { return Quoted{text: s.enquote()} }

// JSON renders s as JSON text.
func (s String) JSON() string { return string(s.enquote()) }

// Text returns s as a plain string. It is shorthand for a type conversion.
func (s String) String() string { return string(s) }

func (s String) enquote() []byte {
	// We might need to reallocate once, but usually not.
	esc := escape.Quote(mem.S(string(s)))
	buf := make([]byte, len(esc)+2)
	copy(buf[1:], esc)
	buf[0] = '"'
	buf[len(buf)-1] = '"'
	return buf
}

// Null represents the JSON null constant. The length of Null is defined as 0.
var Null nullValue

type nullValue struct{}

// Len returns the length of null, which is 0.
func (nullValue) Len() int { return 0 }

// JSON renders the value as a JSON null.
func (nullValue) JSON() string { return "null" }
