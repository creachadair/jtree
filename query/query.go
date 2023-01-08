// Package query implements structural queries over JSON values.
//
// A query describes a syntactic substructure of a JSON syntax tree, such as an
// object member, array element, or a path through the tree. Evaluating a query
// against a concrete JSON value traverses the structure described by the query
// and returns the resulting value.
//
// The simplest query is for a "path", a sequence of object keys and/or array
// indices that describes a path from the root of a JSON value. For example,
// given the JSON value:
//
//	[{"a": 1, "b": 2}, {"c": {"d": true}, "e": false}]
//
// the query
//
//	query.Path(1, "c", "d")
//
// yields the value "true".
package query

import (
	"errors"
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

// Path traverses a sequence of nested object keys or array indices from the
// root.  If no keys are specified, the root is returned. Each key must be a
// string, an int, or a Query.
func Path(keys ...any) Query {
	if len(keys) == 1 {
		return pathElem(keys[0])
	}
	pq := make(Seq, 0, len(keys))
	for _, key := range keys {
		q := pathElem(key)
		if sq, ok := q.(Seq); ok {
			pq = append(pq, sq...)
		} else {
			pq = append(pq, q)
		}
	}
	return pq
}

func pathElem(key any) Query {
	switch t := key.(type) {
	case string:
		return objKey(t)
	case int:
		return nthQuery(t)
	case Query:
		return t
	default:
		panic("invalid path element")
	}
}

type objKey string

func (o objKey) eval(v ast.Value) (ast.Value, error) {
	obj, ok := v.(ast.Object)
	if !ok {
		return nil, fmt.Errorf("got %T, want object", v)
	}
	mem := obj.Find(string(o))
	if mem == nil {
		return nil, fmt.Errorf("key %q not found", o)
	}
	return mem.Value, nil
}

type nthQuery int

func (nq nthQuery) eval(v ast.Value) (ast.Value, error) {
	arr, ok := v.(ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	idx := int(nq)
	if idx < 0 {
		idx += len(arr)
	}
	if idx < 0 || idx >= len(arr) {
		return nil, fmt.Errorf("index %d out of range (0..%d)", nq, len(arr))
	}
	return arr[idx], nil
}

// Selection constructs an array of the elements of its input array, for which
// the specified function returns true.
type Selection func(ast.Value) bool

func (q Selection) eval(v ast.Value) (ast.Value, error) {
	a, ok := v.(ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	var out ast.Array
	for _, elt := range a {
		if q(elt) {
			out = append(out, elt)
		}
	}
	return out, nil
}

// Mapping constructs an array in which each value is replaced by the result of
// calling the specified function on the corresponding input value.
type Mapping func(ast.Value) ast.Value

func (q Mapping) eval(v ast.Value) (ast.Value, error) {
	a, ok := v.(ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	out := make(ast.Array, len(a))
	for i, elt := range a {
		out[i] = q(elt)
	}
	return out, nil
}

// Slice selects a slice of an array from offsets lo to hi.  The range includes
// lo but excludes hi. Negative offsets select from the end of the array.
// If hi == 0, the length of the array is used.
func Slice(lo, hi int) Query { return sliceQuery{lo, hi} }

type sliceQuery struct{ lo, hi int }

func (q sliceQuery) eval(v ast.Value) (ast.Value, error) {
	arr, ok := v.(ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	lox := q.lo
	if lox < 0 {
		lox += len(arr)
	}
	hix := q.hi
	if hix <= 0 {
		hix += len(arr)
	}
	if lox < 0 || lox >= len(arr) {
		return nil, fmt.Errorf("index %d out of range (0..%d)", q.lo, len(arr))
	} else if hix < 0 || hix > len(arr) {
		return nil, fmt.Errorf("index %d out of range (0..%d)", q.hi, len(arr))
	} else if lox > hix {
		return nil, fmt.Errorf("index start %d > end %d", q.lo, q.hi)
	}
	return arr[lox:hix], nil
}

// Pick constructs an array by picking the designated offsets from an array.
// Negative offsets select from the end of the input array.
func Pick(offsets ...int) Query { return pickQuery(offsets) }

type pickQuery []int

func (q pickQuery) eval(v ast.Value) (ast.Value, error) {
	arr, ok := v.(ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	var out ast.Array
	for _, off := range q {
		if off < 0 {
			off += len(arr)
		}
		if off < 0 || off >= len(arr) {
			return nil, fmt.Errorf("index %d out of range (0..%d)", off, len(arr))
		}
		out = append(out, arr[off])
	}
	return out, nil
}

// Len returns an integer representing the length of the root.
//
// For an object, the length is the number of members.
// For an array, the length is the number of elements.
// For a string, the length is the length of the string.
// For null, the length is zero.
func Len() Query { return lenQuery{} }

type lenQuery struct{}

func (lenQuery) eval(v ast.Value) (ast.Value, error) {
	if t, ok := v.(interface {
		Len() int
	}); ok {
		return ast.Int(t.Len()), nil
	}
	return nil, fmt.Errorf("cannot take length of %T", v)
}

// Seq is a sequential composition of queries. An empty sequence selects the
// root; otherwise, each query is applied to the result selected by the
// previous query in the sequence.
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

// Alt is a query that selects among a sequence of alternatives.  The result of
// the first alternative that does not report an error is returned. If there
// are no alternatives, the query fails on all inputs.
type Alt []Query

func (q Alt) eval(v ast.Value) (ast.Value, error) {
	for _, alt := range q {
		if w, err := alt.eval(v); err == nil {
			return w, nil
		}
	}
	return nil, errors.New("no matching alternatives")
}

// Recur applies a query to each recursive descendant of its input and returns
// an array of the resulting values. The arguments have the same constraints as
// Path.
func Recur(keys ...any) Query { return recQuery{Path(keys...)} }

type recQuery struct{ Query }

func (q recQuery) eval(v ast.Value) (ast.Value, error) {
	var out ast.Array

	stk := []ast.Value{v}
	for len(stk) != 0 {
		next := stk[len(stk)-1]
		stk = stk[:len(stk)-1]

		if r, err := q.Query.eval(next); err == nil {
			out = append(out, r)
		}

		// N.B. Push in reverse order, so we visit in lexical order.
		switch t := next.(type) {
		case ast.Object:
			for i := len(t) - 1; i >= 0; i-- {
				stk = append(stk, t[i].Value)
			}
		case ast.Array:
			for i := len(t) - 1; i >= 0; i-- {
				stk = append(stk, t[i])
			}
		}
	}

	if len(out) == 0 {
		return nil, errors.New("no matches")
	}
	return out, nil
}

// Each applies a query to each element of an array and returns an array of the
// resulting values. It fails if the input is not an array.  The arguments have
// the same constraints as Path.
func Each(keys ...any) Query { return eachQuery{Path(keys...)} }

type eachQuery struct{ Query }

func (q eachQuery) eval(v ast.Value) (ast.Value, error) {
	arr, ok := v.(ast.Array)
	if !ok {
		return nil, fmt.Errorf("got %T, want array", v)
	}
	var out ast.Array
	for i, elt := range arr {
		v, err := q.Query.eval(elt)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out = append(out, v)
	}
	return out, nil
}

// Object constructs an object with the given keys mapped to the results of
// matching the query values against its input.
type Object map[string]Query

func (o Object) eval(v ast.Value) (ast.Value, error) {
	var out ast.Object
	for key, q := range o {
		val, err := q.eval(v)
		if err != nil {
			return nil, fmt.Errorf("match %q: %w", key, err)
		}
		out = append(out, ast.Field(key, val))
	}
	return out, nil
}

// Array constructs an array with the values produced by matching the given
// queries against its input.
type Array []Query

func (a Array) eval(v ast.Value) (ast.Value, error) {
	out := make(ast.Array, len(a))
	for i, q := range a {
		val, err := q.eval(v)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out[i] = val
	}
	return out, nil
}

// A String query ignores its input and returns the given string.
func String(s string) Query { return Value(ast.String(s)) }

// A Float query ignores its input and returns the given number.
func Float(n float64) Query { return Value(ast.Float(n)) }

// An Int query ignores its input and returns the given integer.
func Int(z int64) Query { return Value(ast.Int(z)) }

// A Bool query ignores its input and returns the given bool.
func Bool(b bool) Query { return Value(ast.Bool(b)) }

// A Null query ignores its input and returns a null value.
func Null() Query { return Value(ast.Null) }

// A Value query ignores its input and returns the given value.
func Value(v ast.Value) Query { return constQuery{v} }

type constQuery struct{ ast.Value }

func (c constQuery) eval(_ ast.Value) (ast.Value, error) { return c.Value, nil }

// A Glob query returns an array of all its inputs.
func Glob() Query { return globQuery{} }

type globQuery struct{}

func (globQuery) eval(v ast.Value) (ast.Value, error) {
	switch t := v.(type) {
	case ast.Object:
		out := make(ast.Array, len(t))
		for i, m := range t {
			out[i] = m.Value
		}
		return out, nil
	case ast.Array:
		return t, nil
	default:
		return nil, errors.New("no matching values")
	}
}
