package tq

import (
	"errors"
	"fmt"

	"github.com/creachadair/jtree/ast"
)

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
	return with[ast.Object](v, func(obj ast.Object) (ast.Value, error) {
		mem := obj.Find(string(o))
		if mem == nil {
			return nil, fmt.Errorf("key %q not found", o)
		}
		return mem.Value, nil
	})
}

type nthQuery int

func (nq nthQuery) eval(v ast.Value) (ast.Value, error) {
	return with[ast.Array](v, func(a ast.Array) (ast.Value, error) {
		idx := int(nq)
		if idx < 0 {
			idx += len(a)
		}
		if idx < 0 || idx >= len(a) {
			return nil, fmt.Errorf("index %d out of range (0..%d)", nq, len(a))
		}
		return a[idx], nil
	})
}

type sliceQuery struct{ lo, hi int }

func (q sliceQuery) eval(v ast.Value) (ast.Value, error) {
	return with[ast.Array](v, func(arr ast.Array) (ast.Value, error) {
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
	})
}

type pickQuery []int

func (q pickQuery) eval(v ast.Value) (ast.Value, error) {
	return with[ast.Array](v, func(arr ast.Array) (ast.Value, error) {
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
	})
}

type eachQuery struct{ Query }

func (q eachQuery) eval(v ast.Value) (ast.Value, error) {
	return with[ast.Array](v, func(a ast.Array) (ast.Value, error) {
		var out ast.Array
		for i, elt := range a {
			v, err := q.Query.eval(elt)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			out = append(out, v)
		}
		return out, nil
	})
}

type lenQuery struct{}

func (lenQuery) eval(v ast.Value) (ast.Value, error) {
	if t, ok := v.(interface {
		Len() int
	}); ok {
		return ast.Int(t.Len()), nil
	}
	return nil, fmt.Errorf("cannot take length of %T", v)
}

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

type constQuery struct{ ast.Value }

func (c constQuery) eval(_ ast.Value) (ast.Value, error) { return c.Value, nil }

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

func with[T ast.Value](v ast.Value, f func(T) (ast.Value, error)) (ast.Value, error) {
	if v, ok := v.(T); ok {
		return f(v)
	}
	var zero T
	return nil, fmt.Errorf("got %T, want %T", v, zero)
}
