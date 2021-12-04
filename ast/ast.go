// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

// Package ast defines an abstract syntax tree for JSON values,
// and a parser that constructs syntax trees from JSON source.
package ast

import (
	"strconv"

	"github.com/creachadair/jtree"
)

// A Value is an arbitrary JSON value.
type Value interface{ astValue() }

// A Datum is a Value with a text representation.
type Datum interface {
	Value
	Text() string
}

// An Object is a collection of key-value members.
type Object struct {
	Members []*Member
}

func (o Object) astValue() {}

// Find returns the first member of o with the given key, or nil.
func (o Object) Find(key string) *Member {
	for _, m := range o.Members {
		if m.Key() == key {
			return m
		}
	}
	return nil
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

func (m Member) astValue() {}

// Key returns the key of the member.
func (m Member) Key() string {
	if m.dkey != "" {
		return m.dkey
	} else if len(m.key) == 0 {
		return ""
	}
	dec, err := jtree.Unescape(m.key)
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

func (a Array) astValue() {}

type datum struct{ text []byte }

func (d datum) astValue() {}

// Text satisfies the Datum interface.
func (d datum) Text() string { return string(d.text) }

// An Integer is an integer value.
type Integer struct {
	datum
	value *int64
}

// NewInteger constructs an Integer token with the given value.
func NewInteger(z int64) *Integer { return &Integer{value: &z} }

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

// A Number is a floating-point value.
type Number struct {
	datum
	value *float64
}

// NewNumber constructs a Number token with the given value.
func NewNumber(f float64) *Number { return &Number{value: &f} }

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

// A Bool is a Boolean constant, true or false.
type Bool struct {
	datum
	value bool
}

// NewBool constructs a Bool token with the given value.
func NewBool(v bool) *Bool { return &Bool{value: v} }

func (b Bool) Value() bool { return b.value }

// A String is a string value.
type String struct {
	datum
	unescaped []byte
}

// NewString constructs a String token with the given unescaped value.
func NewString(s string) *String { return &String{unescaped: []byte(s)} }

func (s String) Unescape() string {
	if s.unescaped == nil && len(s.text) != 0 {
		dec, err := jtree.Unescape(s.text)
		if err != nil {
			panic(err)
		}
		s.unescaped = dec
	}
	return string(s.unescaped)
}

// Null represents the null constant.
type Null struct{ datum }
