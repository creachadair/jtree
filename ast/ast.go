// Package ast defines an abstract syntax tree for JSON values,
// and a parser that constructs syntax trees from JSON source.
package ast

import "github.com/creachadair/jtree"

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

func (m Member) Span() jtree.Span { return newSpan(m.pos, m.end) }

// An Array is a sequence of values.
type Array struct {
	pos, end int

	Values []Value
}

func (a Array) Span() jtree.Span { return newSpan(a.pos, a.end) }

type datum struct {
	pos, end int
	text     string
}

func (d datum) Span() jtree.Span { return newSpan(d.pos, d.end) }

func (d datum) Text() string { return d.text }

// An Integer is an integer value.
type Integer struct {
	datum
	Value int64
}

// A Number is a floating-point value.
type Number struct {
	datum
	Value float64
}

// A Bool is a Boolean constant, true or false.
type Bool struct {
	datum
	Value bool
}

// A String is a string value.
type String struct {
	datum
	Value string
}

// Null represents the null constant.
type Null struct {
	datum
}
