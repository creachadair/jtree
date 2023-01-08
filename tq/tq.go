// Package tq implements structural traversal queries over JSON values.
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
//	tq.Path(1, "c", "d")
//
// yields the value "true".
package tq

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

// A Query describes a traversal of a JSON value. The behavior of a query is
// defined in terms of how it maps its input to an output. Both the input and
// the output are JSON structures.
type Query interface {
	eval(ast.Value) (ast.Value, error)
}

// Path traverses a sequence of nested object keys or array indices from the
// input value.  If no keys are specified, the input is returned. Each key must
// be a string (an object key), an int (an array offset), or a nested Query.
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

// Selection constructs an array of the elements of its input array for which
// the specified function returns true.
type Selection func(ast.Value) bool

func (q Selection) eval(v ast.Value) (ast.Value, error) {
	return with[ast.Array](v, func(a ast.Array) (ast.Value, error) {
		var out ast.Array
		for _, elt := range a {
			if q(elt) {
				out = append(out, elt)
			}
		}
		return out, nil
	})
}

// Mapping constructs an array in which each value is replaced by the result of
// calling the specified function on the corresponding input value.
type Mapping func(ast.Value) ast.Value

func (q Mapping) eval(v ast.Value) (ast.Value, error) {
	return with[ast.Array](v, func(a ast.Array) (ast.Value, error) {
		out := make(ast.Array, len(a))
		for i, elt := range a {
			out[i] = q(elt)
		}
		return out, nil
	})
}

// Slice selects a slice of an array from offsets lo to hi.  The range includes
// lo but excludes hi. Negative offsets select from the end of the array.
// If hi == 0, the length of the array is used.
func Slice(lo, hi int) Query { return sliceQuery{lo, hi} }

// Pick constructs an array by picking the designated offsets from an array.
// Negative offsets select from the end of the input array.
func Pick(offsets ...int) Query { return pickQuery(offsets) }

// Len returns an integer representing the length of the root.
//
// For an object, the length is the number of members.
// For an array, the length is the number of elements.
// For a string, the length is the length of the string.
// For null, the length is zero.
func Len() Query { return lenQuery{} }

// Seq is a sequential composition of queries. An empty sequence selects the
// input value; otherwise, each query is applied to the result produced by the
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

// Alt is a query that selects among a sequence of alternatives.  It returns
// the value of the first alternative that does not report an error. If there
// are no such alternatives, the query fails. An empty All fails on all inputs.
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

// Each applies a query to each element of an array and returns an array of the
// resulting values. It fails if the input is not an array.  The arguments have
// the same constraints as Path.
func Each(keys ...any) Query { return eachQuery{Path(keys...)} }

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

// Array constructs an array containing the values produced by matching the
// given queries against its input.
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

// A Value query ignores its input and returns the given value.  The value must
// be a string, int, float, bool, nil, or ast.Value.
func Value(v any) Query {
	switch t := v.(type) {
	case string:
		return constQuery{ast.String(t)}
	case int:
		return constQuery{ast.Int(t)}
	case int64:
		return constQuery{ast.Int(t)}
	case float64:
		return constQuery{ast.Float(t)}
	case bool:
		return constQuery{ast.Bool(t)}
	case ast.Value:
		return constQuery{t}
	case nil:
		return constQuery{ast.Null}
	default:
		panic(fmt.Sprintf("invalid constant %T", v))
	}
}

// A Glob query returns an array of its inputs. If the input is an array, the
// array is returned unchanged. if the input is an object, the result is an
// array of all the object values.
func Glob() Query { return globQuery{} }
