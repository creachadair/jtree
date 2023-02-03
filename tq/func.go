package tq

import (
	"errors"

	"github.com/creachadair/jtree/ast"
)

// Is returns its input if it has type T; otherwise it fails.
func Is[T ast.Value]() Query { return Match[T](func(T) bool { return true }) }

// IsNot returns its input if it does not have type T; otherwise it fails.
func IsNot[T ast.Value]() Query {
	return Func(func(_ Env, v ast.Value) (ast.Value, error) {
		if _, ok := v.(T); ok {
			return nil, errors.New("value type does not match")
		}
		return v, nil
	})
}

// Match returns its input if it has the specified type and f reports true for
// its value. Otherwise, the query fails.
func Match[T ast.Value](f func(T) bool) Query {
	return Func(func(_ Env, v ast.Value) (ast.Value, error) {
		w, ok := v.(T)
		if ok && f(w) {
			return v, nil
		}
		return nil, errors.New("value does not match")
	})
}
