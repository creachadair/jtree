// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/creachadair/jtree/ast"
)

// An Array is a commented array of values.
type Array struct {
	Values []Value

	com Comments
}

func (a *Array) Comments() *Comments { return &a.com }

func (a *Array) Undecorate() ast.Value {
	out := make(ast.Array, len(a.Values))
	for i, v := range a.Values {
		out[i] = v.Undecorate()
	}
	return out
}

func (a Array) JSON() string {
	if len(a.Values) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(a.Values[0].JSON())
	for _, elt := range a.Values[1:] {
		sb.WriteByte(',')
		sb.WriteString(elt.JSON())
	}
	sb.WriteByte(']')
	return sb.String()
}

func (a Array) String() string { return fmt.Sprintf("Array(len=%d)", len(a.Values)) }

func (a Array) Len() int { return len(a.Values) }

// A Datum is a commented base value; a string, number, Boolean, or null.
type Datum struct {
	ast.Value

	com Comments
}

func (d *Datum) Comments() *Comments { return &d.com }

func (d *Datum) Undecorate() ast.Value { return d.Value }

// A Document is a single value with optional trailing comments.
type Document struct {
	Value

	com Comments
}

func (d *Document) Comments() *Comments { return &d.com }

func (d *Document) Undecorate() ast.Value { return d.Value.Undecorate() }

// A Member is a key-value pair in an object.
type Member struct {
	Key   ast.Text
	Value Value

	com Comments
}

func (m *Member) Comments() *Comments { return &m.com }

func (m *Member) Undecorate() ast.Value {
	return &ast.Member{Key: m.Key, Value: m.Value.Undecorate()}
}

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

// Field constructs an object member with the given key and value.
// The value must be a string, int, float, bool, nil, or jwcc.Value.
func Field(key string, value any) *Member {
	return &Member{Key: ast.String(key), Value: ToValue(value)}
}

// An Object is a collection of key-value members.
type Object struct {
	Members []*Member

	com Comments
}

func (o *Object) Comments() *Comments { return &o.com }

func (o *Object) Undecorate() ast.Value {
	out := make(ast.Object, len(o.Members))
	for i, m := range o.Members {
		out[i] = m.Undecorate().(*ast.Member)
	}
	return out
}

// FindKey returns the first member of o for whose key f reports true, or nil.
func (o *Object) FindKey(f func(ast.Text) bool) *Member {
	if i := o.IndexKey(f); i >= 0 {
		return o.Members[i]
	}
	return nil
}

// Find is shorthand for FindKey with a case-insensitive name match on key.
func (o *Object) Find(key string) *Member { return o.FindKey(ast.TextEqualFold(key)) }

// IndexKey returns the index of the first member of o for whose key f reports
// true, or -1.
func (o *Object) IndexKey(f func(ast.Text) bool) int {
	for i, m := range o.Members {
		if f(m.Key) {
			return i
		}
	}
	return -1
}

func (o Object) Len() int { return len(o.Members) }

func (o Object) JSON() string {
	if len(o.Members) == 0 {
		return "{}"
	}
	var sb strings.Builder
	sb.WriteByte('{')
	sb.WriteString(o.Members[0].JSON())
	for _, elt := range o.Members[1:] {
		sb.WriteByte(',')
		sb.WriteString(elt.JSON())
	}
	sb.WriteByte('}')
	return sb.String()
}

func (o Object) String() string { return fmt.Sprintf("Object(len=%d)", len(o.Members)) }

// Sort sorts the object in ascending order by key.
func (o Object) Sort() {
	sort.Slice(o.Members, func(i, j int) bool {
		return o.Members[i].Key.String() < o.Members[j].Key.String()
	})
}

// commentStub is a stack placeholder for a comment seen during parsing.
// This type does not appear in a completed AST.
type commentStub struct {
	Value // placeholder, not used

	text        string
	first, last int
}

// arrayStub is a stack placeholder for an incomplete array during parsing.
// This type does not appear in a completed AST.
type arrayStub struct {
	Value // placeholder, not used

	com Comments
}

func (a *arrayStub) Comments() *Comments { return &a.com }

// objectStub is a stack placeholder for an incomplete object during parsing.
// This type does not appear in a completed AST.
type objectStub struct {
	Value // placeholder, not used

	com Comments
}

func (o *objectStub) Comments() *Comments { return &o.com }

// ToValue converts a string, int, float, bool, nil, or ast.Value into a
// jwcc.Value. It panics if v does not have one of those types.
func ToValue(v any) Value {
	if t, ok := v.(Value); ok {
		return t
	}
	return &Datum{Value: ast.ToValue(v)}
}
