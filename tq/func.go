package tq

import "github.com/creachadair/jtree/ast"

// Exists returns a filter that reports true if its argument satisfies the
// specified query. The arguments have the same constraints as Path.
func Exists(keys ...any) FilterFunc {
	q := Path(keys...)
	return func(v ast.Value) bool {
		_, err := q.eval(nil, v)
		return err == nil
	}
}

// Is returns a filter that reports true if its argument is of type T.
func Is[T ast.Value]() FilterFunc {
	return func(v ast.Value) bool { _, ok := v.(T); return ok }
}

// IsNot returns a selection that reports true if its argument is not of type T
func IsNot[T ast.Value]() FilterFunc {
	return func(v ast.Value) bool { _, ok := v.(T); return !ok }
}

// Filter constructs a filter from the given function. The resulting filter
// will discard any value whose type does not match T.
func Filter[T ast.Value](f func(T) bool) FilterFunc {
	return func(v ast.Value) bool { w, ok := v.(T); return ok && f(w) }
}
