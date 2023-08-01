package jwcc

import (
	"fmt"
	"strings"

	"github.com/creachadair/jtree"
	"github.com/creachadair/jtree/ast"
)

// An Array is a commented array of values.
type Array struct {
	Values []Value

	com Comments
}

func (a *Array) Comments() *Comments { return &a.com }

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

// A Datum is a commented base value; a string, number, Boolean, or null.
type Datum struct {
	ast.Value

	com Comments
}

func (d *Datum) Comments() *Comments { return &d.com }

// A Document is a single value with optional trailing comments.
type Document struct {
	Value

	com Comments
}

func (d *Document) Comments() *Comments { return &d.com }

// A Member is a key-value pair in an object.
type Member struct {
	Key   ast.Keyer
	Value Value

	com Comments
}

func (m *Member) Comments() *Comments { return &m.com }

func (m Member) JSON() string {
	k := jtree.Quote(m.Key.Key())
	v := m.Value.JSON()
	buf := make([]byte, len(k)+len(v)+1)
	n := copy(buf, k)
	buf[n] = ':'
	copy(buf[n+1:], v)
	return string(buf)
}

func (m Member) String() string { return fmt.Sprintf("Member(key=%q)", m.Key) }

// An Object is a collection of key-value members.
type Object struct {
	Members []*Member

	com Comments
}

func (o *Object) Comments() *Comments { return &o.com }

func (o *Object) Find(key string) *Member {
	for _, m := range o.Members {
		if m.Key.Key() == key {
			return m
		}
	}
	return nil
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
