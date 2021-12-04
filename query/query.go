// Package query implements structural queries over JSON values.
package query

import (
	"fmt"

	"github.com/creachadair/jtree/ast"
)

// Eval evaluates the given query beginning from root, returning the resulting
// value or an error.
func Eval(root ast.Value, q Query) (ast.Value, error) {
	return q.eval(root)
}

// A Query describes a traversal of a JSON value.
type Query interface {
	eval(ast.Value) (ast.Value, error)
}

// Key traverses a sequence of nested object keys from the root.  If no keys
// are specified, the root is returned.
func Key(keys ...string) Query { return keyQuery(keys) }

type keyQuery []string

func (kq keyQuery) eval(v ast.Value) (ast.Value, error) {
	for _, key := range kq {
		obj, ok := v.(*ast.Object)
		if !ok {
			return nil, fmt.Errorf("got %T, want object", v)
		}
		mem := obj.Find(key)
		if mem == nil {
			return nil, fmt.Errorf("key %q not found", key)
		}
		v = mem.Value
	}
	return v, nil
}

// Index selects the array element at offset z. Negative offsets select from
// the end of the array.
func Index(z int) Query { return indexQuery(z) }

type indexQuery int

func (iq indexQuery) eval(v ast.Value) (ast.Value, error) {
	arr, ok := v.(*ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	idx := int(iq)
	if idx < 0 {
		idx += len(arr.Values)
	}
	if idx < 0 || idx >= len(arr.Values) {
		return nil, fmt.Errorf("index %d out of range (0..%d)", iq, len(arr.Values))
	}
	return arr.Values[idx], nil
}

// Len selects an Integer representing the length of the root.
// For an object, the length is the number of members.
// For an array, the length is the number of elements.
// For a string, the length is the length of the string.
// For null, the length is zero.
func Len() Query { return lenQuery{} }

type lenQuery struct{}

func (lenQuery) eval(v ast.Value) (ast.Value, error) {
	switch t := v.(type) {
	case *ast.Object:
		return ast.NewInteger(int64(len(t.Members))), nil
	case *ast.Array:
		return ast.NewInteger(int64(len(t.Values))), nil
	case *ast.Null:
		return ast.NewInteger(0), nil
	case *ast.String:
		return ast.NewInteger(int64(len(t.Unescape()))), nil
	default:
		return nil, fmt.Errorf("cannot take length of %T", v)
	}
}

// Seq is a sequential composition of queries. An empty Seq selects the root;
// otherwise, each query is applied to the result selected by the previous
// query in the sequence.
type Seq []Query

func (q Seq) eval(v ast.Value) (ast.Value, error) {
	cur := v
	for _, sq := range q {
		next, err := sq.eval(cur)
		if err != nil {
			return nil, err
		}
		cur = next
	}
	return cur, nil
}

// Each applies a query to each element of an array and returns an array of the
// resulting values. It fails if the input is not an array.
func Each(q Query) Query { return eachQuery{q} }

type eachQuery struct{ Query }

func (q eachQuery) eval(v ast.Value) (ast.Value, error) {
	arr, ok := v.(*ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	out := new(ast.Array)
	for i, elt := range arr.Values {
		v, err := q.Query.eval(elt)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out.Values = append(out.Values, v)
	}
	return out, nil
}

// Object constructs an object with the given keys mapped to the results of
// matching the query values against its input.
type Object map[string]Query

func (o Object) eval(v ast.Value) (ast.Value, error) {
	out := new(ast.Object)
	for key, q := range o {
		val, err := q.eval(v)
		if err != nil {
			return nil, fmt.Errorf("match %q: %w", key, err)
		}
		out.Members = append(out.Members, ast.NewMember(key, val))
	}
	return out, nil
}

// Array constructs an array with the values produced by matching the given
// queries against its input.
type Array []Query

func (a Array) eval(v ast.Value) (ast.Value, error) {
	out := &ast.Array{Values: make([]ast.Value, len(a))}
	for i, q := range a {
		val, err := q.eval(v)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out.Values[i] = val
	}
	return out, nil
}
