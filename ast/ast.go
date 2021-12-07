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

// An Object is a collection of key-value members.
type Object struct {
	Members []*Member
}

func (o *Object) astValue() {}

// Find returns the first member of o with the given key, or nil.
func (o *Object) Find(key string) *Member {
	for _, m := range o.Members {
		if m.Key() == key {
			return m
		}
	}
	return nil
}

// String renders o as JSON text.
func (o *Object) String() string {
	var sb strings.Builder
	sb.WriteString("{")
	last := len(o.Members) - 1
	for i, elt := range o.Members {
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
	key  []byte
	dkey string

	Value Value
}

// NewMember constructs a member with the given key and value.
func NewMember(key string, val Value) *Member {
	return &Member{dkey: key, Value: val}
}

func (m *Member) astValue() {}

// String renders the member as JSON text.
func (m *Member) String() string {
	// FIXME: Handle constructed strings.
	return string(m.key) + ":" + m.Value.String()
}

// Key returns the key of the member.
func (m *Member) Key() string {
	if m.dkey != "" {
		return m.dkey
	} else if len(m.key) == 0 {
		return ""
	}
	dec, err := jtree.Unescape(m.key[1 : len(m.key)-1])
	if err != nil {
		panic(err)
	}
	m.dkey = string(dec)
	return m.dkey
}

// An Array is a sequence of values.
type Array struct {
	Values []Value
}

func (a *Array) astValue() {}

// String renders the array as JSON text.
func (a *Array) String() string {
	var sb strings.Builder
	sb.WriteString("[")
	last := len(a.Values) - 1
	for i, elt := range a.Values {
		sb.WriteString(elt.String())
		if i != last {
			sb.WriteString(",")
		}
	}
	sb.WriteString("]")
	return sb.String()
}

// An Integer is an integer value.
type Integer struct {
	text  []byte
	value *int64
}

// NewInteger constructs an Integer token with the given value.
func NewInteger(z int64) *Integer { return &Integer{value: &z} }

func (*Integer) astValue() {}

// Int64 returns the value of z as an int64.
func (z *Integer) Int64() int64 {
	if z.value == nil {
		v, err := strconv.ParseInt(string(z.text), 10, 64)
		if err != nil {
			panic(err)
		}
		z.value = &v
	}
	return *z.value
}

// String renders z as JSON text.
func (z *Integer) String() string {
	if z.text != nil {
		return string(z.text)
	}
	return strconv.FormatInt(*z.value, 10)
}

// A Number is a floating-point value.
type Number struct {
	text  []byte
	value *float64
}

// NewNumber constructs a Number token with the given value.
func NewNumber(f float64) *Number { return &Number{value: &f} }

func (*Number) astValue() {}

// Float64 returns the value of n as a float64.
func (n *Number) Float64() float64 {
	if n.value == nil {
		v, err := strconv.ParseFloat(string(n.text), 64)
		if err != nil {
			panic(err)
		}
		n.value = &v
	}
	return *n.value
}

// String renders n as JSON text.
func (n *Number) String() string {
	if n.text != nil {
		return string(n.text)
	}
	return strconv.FormatFloat(*n.value, 'g', -1, 64)
}

// A Bool is a Boolean constant, true or false.
type Bool struct {
	value bool
}

// NewBool constructs a Bool token with the given value.
func NewBool(v bool) *Bool { return &Bool{value: v} }

// Value reports the truth value of the Boolean.
func (b *Bool) Value() bool { return b.value }

func (*Bool) astValue() {}

// String returns b as JSON text.
func (b *Bool) String() string {
	if b.value {
		return "true"
	}
	return "false"
}

// A String is a string value.
type String struct {
	text      []byte
	unescaped *string
}

// NewString constructs a String token with the given unescaped value.
func NewString(s string) *String { return &String{unescaped: &s} }

func (*String) astValue() {}

// Unescape returns the unescaped text of the string.
func (s *String) Unescape() string {
	if s.unescaped == nil && len(s.text) != 0 {
		dec, err := jtree.Unescape(s.text[1 : len(s.text)-1])
		if err != nil {
			panic(err)
		}
		str := string(dec)
		s.unescaped = &str
	}
	return *s.unescaped
}

// String renders s as JSON text.
func (s *String) String() string {
	// FIXME: Handle constructed strings.
	return string(s.text)
}

// Null represents the null constant.
type Null struct{}

func (Null) astValue() {}

// String renders the value as a JSON null.
func (Null) String() string { return "null" }
