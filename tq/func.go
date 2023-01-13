package tq

import "github.com/creachadair/jtree/ast"

// Exists returns a selection that reports true if its argument satisfies the
// specified query. The arguments have the same constraints as Path.
func Exists(keys ...any) Selection {
	q := Path(keys...)
	return func(v ast.Value) bool {
		_, err := q.eval(nil, v)
		return err == nil
	}
}

// Is returns a selection that reports true if its argument is of type T.
func Is[T ast.Value]() Selection {
	return func(v ast.Value) bool { _, ok := v.(T); return ok }
}

// IsNot returns a selection that reports true if its argument is not of type T
func IsNot[T ast.Value]() Selection {
	return func(v ast.Value) bool { _, ok := v.(T); return !ok }
}

// Select constructs a selection from the given function. The resulting
// selection will discard any value whose type does not match T.
func Select[T ast.Value](f func(T) bool) Selection {
	return func(v ast.Value) bool { w, ok := v.(T); return ok && f(w) }
}
