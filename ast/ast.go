// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

// Package ast defines an abstract syntax tree for JSON values,
// and a parser that constructs syntax trees from JSON source.
package ast

import (
	"strconv"

	"github.com/creachadair/jtree"
)

// A Value is an arbitrary JSON value.
type Value interface{ Span() jtree.Span }

// A Datum is a Value with a text representation.
type Datum interface {
	Value
	Text() string
}

func newSpan(pos, end int) jtree.Span { return jtree.Span{Pos: pos, End: end} }

// An Object is a collection of key-value members.
type Object struct {
	pos, end int
	Members  []*Member
}

// Span satisfies the Value interface.
func (o Object) Span() jtree.Span { return newSpan(o.pos, o.end) }

// Find returns the first member of o with the given key, or nil.
func (o Object) Find(key string) *Member {
	for _, m := range o.Members {
		if m.Key == key {
			return m
		}
	}
	return nil
}

// A Member is a single key-value pair belonging to an Object.
type Member struct {
	pos, end int

	Key   string
	Value Value
}

// Span satisfies the Value interface.
func (m Member) Span() jtree.Span { return newSpan(m.pos, m.end) }

// An Array is a sequence of values.
type Array struct {
	pos, end int

	Values []Value
}

// Span satisfies the Value interface.
func (a Array) Span() jtree.Span { return newSpan(a.pos, a.end) }

type datum struct {
	pos, end int
	text     []byte
}

// Span satisfies the Value interface.
func (d datum) Span() jtree.Span { return newSpan(d.pos, d.end) }

// Text satisfies the Datum interface.
func (d datum) Text() string { return string(d.text) }

// An Integer is an integer value.
type Integer struct{ datum }

func (z Integer) Int64() int64 {
	v, err := strconv.ParseInt(string(z.text), 10, 64)
	if err != nil {
		panic(err)
	}
	return v
}

// A Number is a floating-point value.
type Number struct{ datum }

func (n Number) Float64() float64 {
	v, err := strconv.ParseFloat(string(n.text), 64)
	if err != nil {
		panic(err)
	}
	return v
}

// A Bool is a Boolean constant, true or false.
type Bool struct {
	datum
	value bool
}

func (b Bool) Value() bool { return b.value }

// A String is a string value.
type String struct{ datum }

func (s String) Unescape() string {
	dec, err := jtree.Unescape(s.text)
	if err != nil {
		panic(err)
	}
	return string(dec)
}

// Null represents the null constant.
type Null struct{ datum }
