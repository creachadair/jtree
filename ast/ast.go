// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

// Package ast defines an abstract syntax tree for JSON values,
// and a parser that constructs syntax trees from JSON source.
package ast

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/creachadair/jtree"
	"github.com/creachadair/jtree/internal/escape"
	"go4.org/mem"
)

// A Value is an arbitrary JSON value.
type Value interface {
	// JSON converts the value into JSON source text.
	JSON() string

	// String converts the value into a human-readable string.  The result is
	// not required to be valid JSON.
	String() string
}

// A Number is a Value that represents a number. Number values have the
// property that they can be converted into Int or Float.
type Number interface {
	Value

	IsInt() bool  // reports whether the value is an integer
	Int() Int     // converts the value to an integer
	Float() Float // converts the value to floating point
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

func (o Object) String() string { return fmt.Sprintf("Object(len=%d)", len(o)) }

// Sort sorts the object in ascending order by key.
func (o Object) Sort() {
	sort.Slice(o, func(i, j int) bool { return o[i].Key.String() < o[j].Key.String() })
}

// A Member is a single key-value pair belonging to an Object. A Key must
// support being rendered as text, typically an ast.String.
type Member struct {
	Key   Text
	Value Value
}

// Field constructs an object member with the given key and value.  The value
// must be a string, int, float, bool, nil, or ast.Value.
func Field(key string, value any) *Member {
	return &Member{Key: String(key), Value: ToValue(value)}
}

// ToValue converts a string, int, float, bool, nil, or ast.Value into an
// ast.Value. It panics if v does not have one of those types.
func ToValue(v any) Value {
	switch t := v.(type) {
	case string:
		return String(t)
	case int:
		return Int(t)
	case int64:
		return Int(t)
	case float64:
		return Float(t)
	case bool:
		return Bool(t)
	case nil:
		return Null
	case Value:
		return t
	default:
		panic(fmt.Sprintf("invalid value %T", v))
	}
}

// JSON renders the member as JSON text.
func (m Member) JSON() string {
	k := m.Key.Quote().JSON()
	v := m.Value.JSON()
	buf := make([]byte, len(k)+len(v)+1)
	n := copy(buf, k)
	buf[n] = ':'
	copy(buf[n+1:], v)
	return string(buf)
}

func (m Member) String() string { return fmt.Sprintf("Member(key=%q)", m.Key) }

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

func (a Array) String() string { return fmt.Sprintf("Array(len=%d)", len(a)) }

// A rawNumber is a numeric literal parsed from source txt.
type rawNumber struct {
	text  []byte
	isInt bool // whether the value was lexed as an integer
}

// JSON renders n as JSON text.
func (n rawNumber) JSON() string { return string(n.text) }

func (n rawNumber) String() string { return string(n.text) }

// IsInt reports whether n is representable as an integer.
func (n rawNumber) IsInt() bool { return n.isInt }

// Float returns a representation of n as a Float. It panics if n is not
// representable as a floating-point value.
func (n rawNumber) Float() Float {
	v, err := jtree.ParseFloat(n.text, 64)
	if err != nil {
		panic(err)
	}
	return Float(v)
}

// Int returns a representation of n as an Int.  If n is valid but has
// fractional parts, the fractions are truncated; otherwise Int panics if n is
// not representable as a number.
func (n rawNumber) Int() Int {
	if n.isInt {
		v, err := jtree.ParseInt(n.text, 10, 64)
		if err != nil {
			panic(err)
		}
		return Int(v)
	}
	v, err := jtree.ParseFloat(n.text, 64)
	if err != nil {
		panic(err)
	}
	return Int(v)
}

// A Float is represents a floating-point number.
type Float float64

// Float satisfies the Numeric interface. It returns f unmodified.
func (f Float) Float() Float { return f }

// IsInt reports false for f.
func (Float) IsInt() bool { return false }

// Int satisfies the numeric interface.
func (f Float) Int() Int { return Int(f) }

// JSON renders f as JSON text.
func (f Float) JSON() string { return strconv.FormatFloat(float64(f), 'g', -1, 64) }

func (f Float) String() string { return f.JSON() }

// An Int represents an integer number.
type Int int64

// Int satisfies the Numeric interface. It returns z unmodified.
func (z Int) Int() Int { return z }

// IsInt reports true for z.
func (Int) IsInt() bool { return true }

// Float satisfies the Numeric interface.
func (z Int) Float() Float { return Float(z) }

// JSON renders z as JSON text.
func (z Int) JSON() string { return strconv.FormatInt(int64(z), 10) }

func (z Int) String() string { return z.JSON() }

// A Bool is a Boolean constant, true or false.
type Bool bool

// JSON returns b as JSON text.
func (b Bool) JSON() string {
	if b {
		return "true"
	}
	return "false"
}

func (b Bool) String() string { return b.JSON() }

// Text represents a value that can be encoded as a JSON string.
// The String method of a Text value returns the plain string without quotes.
type Text interface {
	Value

	Quote() Text // returns a quoted representation of the text
}

// A quotedText is a quoted string value.
type quotedText struct{ data mem.RO }

// Quote returns q unmodified to satisfy the Text interface.
func (q quotedText) Quote() Text { return q }

// unquote returns the unescaped text of the string.
func (q quotedText) unquote() string {
	n := q.data.Len()
	if n == 0 {
		return ""
	}
	dec, err := escape.Unquote(q.data.Slice(1, n-1))
	if err != nil {
		panic(err)
	}
	return string(dec)
}

// Quoted constructs a quoted text value for s.
func Quoted(s string) Text { return quotedText{data: mem.S(s)} }

// Len returns the length in bytes of the unquoted text of q.
func (q quotedText) Len() int { return len(q.unquote()) }

// JSON returns the JSON encoding of q.
func (q quotedText) JSON() string { return q.data.StringCopy() }

// String returns the unquoted string represented by q.
func (q quotedText) String() string { return q.unquote() }

// A String is an unquoted text value.
type String string

// Len returns the length in bytes of s.
func (s String) Len() int { return len(s) }

// Quote converts s into its quoted representation.
func (s String) Quote() Text { return quotedText{data: escape.Quote(mem.S(string(s)))} }

// JSON renders s as JSON text.
func (s String) JSON() string { return jtree.Quote(string(s)) }

func (s String) String() string { return string(s) }

// Null represents the JSON null constant. The length of Null is defined as 0.
var Null nullValue

type nullValue struct{}

// Len returns the length of null, which is 0.
func (nullValue) Len() int { return 0 }

// JSON renders the value as a JSON null.
func (nullValue) JSON() string { return "null" }

func (nullValue) String() string { return "null" }
