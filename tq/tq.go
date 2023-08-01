// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

// Package tq implements structural traversal queries over JSON values.
//
// A query describes a syntactic substructure of a JSON document, such as an
// object, array, array element, or basic value. Evaluating a query against a
// concrete JSON value traverses the value as described by the query and
// returns the resulting structure.
//
// The most basic query is for a "path", a sequence of object keys and/or array
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
//
// # Bindings
//
// Queries can save intermediate results in named bindings. This is done using
// the As query, which stores its input under the given name:
//
//	tq.Path(0, "b", tq.As("Q"))
//
// Another part of the query can recover this value using a Get query:
//
//	tq.Get("Q")
//
// Path constructors support the shorthand "$x" for a query like tq.Get("x").
// You can escape this if you want the literal string "$x" by writing "$$x".
//
// The As query returns its input. To bind a subquery based on the input, put
// that subquery into the As query:
//
//	tq.Seq{
//	  tq.As("@", 1, "c"),
//	  tq.Path("$@", "d"),
//	}
//
// yields the value "true".
//
// The special name "$" is pre-bound to the root of the input.
package tq

import (
	"errors"
	"fmt"

	"github.com/creachadair/jtree/ast"
)

// Eval evaluates the given query beginning from root, returning the resulting
// value or an error.
func Eval(root ast.Value, q Query) (ast.Value, error) {
	var empty *qstate
	_, w, err := q.eval(empty.bind("$", root), root)
	return w, err
}

// A Query describes a traversal of a JSON value. The behavior of a query is
// defined in terms of how it maps its input to an output. Both the input and
// the output are JSON structures.
type Query interface {
	eval(*qstate, ast.Value) (*qstate, ast.Value, error)
}

// Path traverses a sequence of nested object keys or array indices from the
// input value.  If no keys are specified, the input is returned. Each key must
// be a string (an object key), an int (an array offset), or a nested Query.
//
// As a special case, a string beginning with "$" is treated as a Get query.
// To escape this treatment, double the "$".
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

// Select constructs an array of elements from its input array whose values
// match the query. The arguments have the same constraints as Path.
func Select(keys ...any) Query { return selectQuery{Path(keys...)} }

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
//
// Parameters bound by As and Func queries evaluated in a sequence are visible
// to later queries in the sequence.
type Seq []Query

func (q Seq) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	cs, cur := qs, v
	for _, sq := range q {
		ns, next, err := sq.eval(cs, cur)
		if err != nil {
			return cs, nil, err
		}
		cs, cur = ns, next
	}
	return cs, cur, nil
}

// Alt is a query that selects among a sequence of alternatives.  It returns
// the value of the first alternative that does not report an error. If there
// are no such alternatives, the query fails. An empty All fails on all inputs.
type Alt []Query

func (q Alt) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	for _, alt := range q {
		// N.B. Evaluate each alternative in qs, so previous attempts do not
		// affect the environment of the next.
		rs, w, err := alt.eval(qs, v)
		if err == nil {
			return rs, w, nil
		}
	}
	return qs, nil, errors.New("no matching alternatives")
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

func (o Object) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	var out ast.Object
	for key, q := range o {
		_, val, err := q.eval(qs, v)
		if err != nil {
			return qs, nil, fmt.Errorf("match %q: %w", key, err)
		}
		out = append(out, ast.Field(key, val))
	}
	return qs, out, nil
}

// Array constructs an array containing the values produced by matching the
// given queries against its input.
type Array []Query

func (a Array) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	out := make(ast.Array, len(a))
	for i, q := range a {
		_, val, err := q.eval(qs, v)
		if err != nil {
			return qs, nil, fmt.Errorf("index %d: %w", i, err)
		}
		out[i] = val
	}
	return qs, out, nil
}

// A Delete query removes the specified key from its input object and returns
// the resulting object. It is an error if the input is not an object, but no
// error is reported if the input lacks that key. A JSON null value is treated
// as an empty object for purposes of this query.
func Delete(name string) Query { return delQuery{name} }

// A Set query adds name to a copy of its input object, with the value from the
// given query evaluated on that input, and returns the resulting object.
//
// If name already exists in the input, its value is replaced in the output. It
// is an error if the input is not an object.  A JSON null value is treated as
// an empty object for purposes of this query.
func Set(name string, keys ...any) Query { return setQuery{name, Path(keys...)} }

// A Value query ignores its input and returns the given value.  The value must
// be a string, int, float, bool, nil, or ast.Value.
func Value(v any) Query { return constQuery{ast.ToValue(v)} }

// A Glob query returns an array of its inputs. If the input is an array, the
// array is returned unchanged. If the input is an object, the result is an
// array of all the object values.
func Glob() Query { return globQuery{} }

// A Keys query returns an array of the keys of an object value. It is an error
// if the input is not an object or null.
func Keys() Query { return keysQuery{} }

// A Get query ignores its input and instead returns the value associated with
// the specified parameter name. The query fails if the name is not defined.
func Get(name string) Query { base, _ := isMarked(name); return getQuery{base} }

// As evaluates the given subquery on its input, then returns its input in an
// environment where name is bound to the result from the subquery. If the
// subquery is empty, the name is bound to the input itself.
func As(name string, keys ...any) Query {
	base, _ := isMarked(name)
	return asQuery{base, Path(keys...)}
}

// Env is the namespace environment for a query.
type Env struct{ *qstate }

// Get returns the value associated with name in environment, or nil if the
// name is not bound.
func (e Env) Get(name string) ast.Value {
	if v, ok := e.qstate.lookup(name); ok {
		return v
	}
	return nil
}

// Bind extends e with a binding for the given name and value.  If the name
// already exists in e, the new definition shadows the previous one.
func (e Env) Bind(name string, value ast.Value) Env {
	return Env{e.qstate.bind(name, value)}
}

// Eval evaluates the specified query starting from v.
func (e Env) Eval(v ast.Value, q Query) (Env, ast.Value, error) {
	rs, w, err := q.eval(e.qstate, v)
	return Env{qstate: rs}, w, err
}

// Func is a user-defined Query implementation. Evaluating the query calls the
// function with the current namespace environment and input value.
type Func func(Env, ast.Value) (Env, ast.Value, error)

func (f Func) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	e, w, err := f(Env{qstate: qs}, v)
	return e.qstate, w, err
}

// Ref returns a query that looks up the string or integer value returned by q
// as an object or array reference on its input. It is an error if the value
// from q is not a string or a number. The parameter q has the same constraints
// as the arguments to Path.
func Ref(q ...any) Query { return refQuery{Path(q...)} }
