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
// the Set query, which stores its input under the given name:
//
//	tq.Path(0, "b", tq.Set("Q"))
//
// Another part of the query can recover this value using a Get query:
//
//	tq.Get("Q")
//
// Path constructors support the shorthand "$q" for a query like tq.Get("x").
// You can escape this if you want the literal string "$x" by writing "$$x".
//
// Bindings are ordinarily visible to the rest of the query after a Set.  The
// Let form can be used to bind a name only for the duration of a subquery.
// For example, this query:
//
//	tq.Let{
//	   {"@", tq.Path(1, "c")},
//	}.In(tq.Get("@"), "d")
//
// yields the value "true", but "@" is not visible to subsequent subqueries.
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
	return q.eval(new(qstate).bind("$", root), root)
}

// A Query describes a traversal of a JSON value. The behavior of a query is
// defined in terms of how it maps its input to an output. Both the input and
// the output are JSON structures.
type Query interface {
	eval(*qstate, ast.Value) (ast.Value, error)
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

// Selection constructs an array of the elements of its input array for which
// the specified function returns true.
type Selection func(ast.Value) bool

func (q Selection) eval(_ *qstate, v ast.Value) (ast.Value, error) {
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

func (q Mapping) eval(_ *qstate, v ast.Value) (ast.Value, error) {
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
//
// Parameters defined during evaluation of a sequence are visible to later
// queries in the sequence. Notably, Set and Func queries can do this.
type Seq []Query

func (q Seq) eval(qs *qstate, v ast.Value) (ast.Value, error) {
	cur := v
	for _, sq := range q {
		next, err := sq.eval(qs, cur)
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

func (q Alt) eval(qs *qstate, v ast.Value) (ast.Value, error) {
	for _, alt := range q {
		// When evaluating alternatives, don't let failed branches contribute to
		// the namespace. Once we find one that succeeds we can copy any bindings
		// it produced.
		ns := qs.push()
		if w, err := alt.eval(ns, v); err == nil {
			qs.bindings = append(qs.bindings, ns.bindings...)
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

func (o Object) eval(qs *qstate, v ast.Value) (ast.Value, error) {
	var out ast.Object
	for key, q := range o {
		val, err := q.eval(qs, v)
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

func (a Array) eval(qs *qstate, v ast.Value) (ast.Value, error) {
	out := make(ast.Array, len(a))
	for i, q := range a {
		val, err := q.eval(qs, v)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		out[i] = val
	}
	return out, nil
}

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

// Let defines a set of name to query bindings. These bindings can be applied
// to a query q using the In method, to evaluate q with the names bound to the
// values defined.
//
// Bindings in a Let are ordered: Each query can see the names of the queries
// prior to it in the sequence.
type Let []Bind

// A Bind associates a name with a query.
type Bind struct {
	Name  string
	Query Query
}

// In applies b to the specified query. The arguments have the same constraints
// as Path.
func (b Let) In(keys ...any) Query { return letQuery{binds: b, next: Path(keys...)} }

// A Get query ignores its input and instead returns the value associated with
// the specified parameter name. The query fails if the name is not defined.
func Get(name string) Query { base, _ := isMarked(name); return getQuery{base} }

// A Set query copies its input to its output, and as a side-effect updates the
// specified parameter name with the input vaoue.
func Set(name string) Query { base, _ := isMarked(name); return setQuery{base} }

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

// Set adds a binding to the environment for the given name. If the name
// already exists, the new definition shadows the previous one.
func (e Env) Set(name string, value ast.Value) { e.qstate.bind(name, value) }

// New derives a new empty environment frame from e. Ordinarily when Func
// evaluates a subquery, any modifications it makes to the environment are
// preserved when the Func completes. A new frame isolates such changes to the
// subquery.
func (e Env) New() Env { return Env{qstate: e.qstate.push()} }

// Eval evaluates the specified query starting from v.
func (e Env) Eval(v ast.Value, q Query) (ast.Value, error) { return q.eval(e.qstate, v) }

// Func is a user-defined Query implementation. Evaluating the query calls the
// function with the current namespace environment and input value.
type Func func(Env, ast.Value) (ast.Value, error)

func (f Func) eval(qs *qstate, v ast.Value) (ast.Value, error) {
	return f(Env{qstate: qs}, v)
}

// Ref returns a query that looks up the string or integer value returned by q
// as an object or array reference on its input. It is an error if the value
// from q is not a string or a number. The parameter q has the same constraints
// as the arguments to Path.
func Ref(q ...any) Query { return refQuery{Path(q...)} }
