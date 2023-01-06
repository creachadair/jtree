// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

// Package ast defines an abstract syntax tree for JSON values,
// and a parser that constructs syntax trees from JSON source.
package ast

import (
	"strconv"
	"strings"

	"github.com/creachadair/jtree"
)

// A Value is an arbitrary JSON value.
type Value interface {
	astValue()
	String() string
}

// A Texter is a Value that can be rendered as a string.
type Texter interface {
	Value
	Text() string
}

// An Object is a collection of key-value members.
type Object []*Member

func (o Object) astValue() {}

// Find returns the first member of o with the given key, or nil.
func (o Object) Find(key string) *Member {
	for _, m := range o {
		if m.Key.Text() == key {
			return m
		}
	}
	return nil
}

// Len returns the number of members in the object.
func (o Object) Len() int { return len(o) }

// String renders o as JSON text.
func (o Object) String() string {
	var sb strings.Builder
	sb.WriteString("{")
	last := len(o) - 1
	for i, elt := range o {
		sb.WriteString(elt.String())
		if i != last {
			sb.WriteString(",")
		}
	}
	sb.WriteString("}")
	return sb.String()
}

// A Member is a single key-value pair belonging to an Object.
type Member struct {
	Key   Texter
	Value Value
}

// Field constructs an object member with the given key and value.
func Field(key string, val Value) *Member {
	return &Member{
		Key:   String(key),
		Value: val,
	}
}

func (m *Member) astValue() {}

// String renders the member as JSON text.
func (m *Member) String() string {
	return m.Key.String() + ":" + m.Value.String()
}

// An Array is a sequence of values.
type Array []Value

func (Array) astValue() {}

// Len returns the number of elements in a.
func (a Array) Len() int { return len(a) }

// String renders the array as JSON text.
func (a Array) String() string {
	if len(a) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(a[0].String())
	for _, elt := range a[1:] {
		sb.WriteByte(',')
		sb.WriteString(elt.String())
	}
	sb.WriteByte(']')
	return sb.String()
}

// A Number is a numeric literal.
type Number struct{ text []byte }

func (Number) astValue() {}

// String renders n as JSON text.
func (n Number) String() string { return string(n.text) }

// Float returns a representation of n as a Float. It panics if n is not
// representable as a floating-point value.
func (n Number) Float() Float {
	v, err := strconv.ParseFloat(string(n.text), 64)
	if err != nil {
		panic(err)
	}
	return Float(v)
}

// Int returns a representation of n as an Int.  If n is valid but has
// fractional parts, the fractions are truncated; otherwise Int panics if n is
// not representable as a number.
func (n Number) Int() Int {
	v, err := strconv.ParseFloat(string(n.text), 64)
	if err != nil {
		panic(err)
	}
	return Int(v)
}

// A Float is represents a floating-point number.
type Float float64

func (Float) astValue() {}

// String renders f as JSON text.
func (f Float) String() string { return strconv.FormatFloat(float64(f), 'g', -1, 64) }

// Value returns f as a float64. It is shorthand for a type conversion.
func (f Float) Value() float64 { return float64(f) }

// An Int represents an integer number.
type Int int64

func (Int) astValue() {}

// String renders z as JSON text.
func (z Int) String() string { return strconv.FormatInt(int64(z), 10) }

// Value returns z as an int64. It is shorthand for a type conversion.
func (z Int) Value() int64 { return int64(z) }

// A Bool is a Boolean constant, true or false.
type Bool bool

// Value reports the truth value of the Boolean.
func (b Bool) Value() bool { return bool(b) }

func (Bool) astValue() {}

// String returns b as JSON text.
func (b Bool) String() string {
	if b {
		return "true"
	}
	return "false"
}

// A Quoted is a quoted string value.
type Quoted struct{ text []byte }

func (Quoted) astValue() {}

// Unquote returns the unescaped text of the string.
func (q Quoted) Unquote() String {
	if len(q.text) == 0 {
		return ""
	}
	dec, err := jtree.Unescape(q.text[1 : len(q.text)-1])
	if err != nil {
		panic(err)
	}
	return String(dec)
}

// Text returns the unescaped text of the string.
func (q Quoted) Text() string { return string(q.Unquote()) }

// Len returns the length in bytes of the unquoted text of q.
func (q Quoted) Len() int { return q.Unquote().Len() }

// String returns the JSON encoding of q.
func (q Quoted) String() string { return string(q.text) }

// A String is an unquoted string value.
type String string

func (String) astValue() {}

// Len returns the length in bytes of s.
func (s String) Len() int { return len(s) }

// Quote converts s into its quoted representation.
func (s String) Quote() Quoted { return Quoted{text: s.enquote()} }

// String renders s as JSON text.
func (s String) String() string { return string(s.enquote()) }

// Text returns s as a plain string. It is shorthand for a type conversion.
func (s String) Text() string { return string(s) }

func (s String) enquote() []byte {
	// We might need to reallocate once, but usually not.
	buf := make([]byte, 0, len(s)+2)
	buf = append(buf, '"')
	buf = append(buf, jtree.Escape(string(s))...)
	return append(buf, '"')
}

// Null represents the JSON null constant. The length of Null is defined as 0.
var Null nullValue

type nullValue struct{}

func (nullValue) astValue() {}

// Len returns the length of null, which is 0.
func (nullValue) Len() int { return 0 }

// String renders the value as a JSON null.
func (nullValue) String() string { return "null" }
