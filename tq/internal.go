// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package tq

import (
	"errors"
	"fmt"
	"strings"

	"github.com/creachadair/jtree/ast"
)

func pathElem(key any) Query {
	switch t := key.(type) {
	case string:
		s, ok := isMarked(t)
		if ok {
			return Get(s)
		}
		return objKey(s)
	case int:
		return nthQuery(t)
	case Query:
		return t
	case ast.Value:
		return Value(t)
	default:
		panic(fmt.Sprintf("invalid path element %T", key))
	}
}

type objKey string

func (o objKey) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	return with(qs, v, func(obj ast.Object) (*qstate, ast.Value, error) {
		mem := obj.Find(string(o))
		if mem == nil {
			return qs, nil, fmt.Errorf("key %q not found", o)
		}
		return qs, mem.Value, nil
	})
}

type nthQuery int

func (nq nthQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	return with(qs, v, func(a ast.Array) (*qstate, ast.Value, error) {
		idx := int(nq)
		if idx < 0 {
			idx += len(a)
		}
		if idx < 0 || idx >= len(a) {
			return qs, nil, fmt.Errorf("index %d out of range (0..%d)", nq, len(a))
		}
		return qs, a[idx], nil
	})
}

type sliceQuery struct{ lo, hi int }

func (q sliceQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	return with(qs, v, func(arr ast.Array) (*qstate, ast.Value, error) {
		lox := q.lo
		if lox < 0 {
			lox += len(arr)
		}
		hix := q.hi
		if hix <= 0 {
			hix += len(arr)
		}
		if lox < 0 || lox >= len(arr) {
			return qs, nil, fmt.Errorf("index %d out of range (0..%d)", q.lo, len(arr))
		} else if hix < 0 || hix > len(arr) {
			return qs, nil, fmt.Errorf("index %d out of range (0..%d)", q.hi, len(arr))
		} else if lox > hix {
			return qs, nil, fmt.Errorf("index start %d > end %d", q.lo, q.hi)
		}
		return qs, arr[lox:hix], nil
	})
}

type pickQuery []int

func (q pickQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	return with(qs, v, func(arr ast.Array) (*qstate, ast.Value, error) {
		var out ast.Array
		for _, off := range q {
			if off < 0 {
				off += len(arr)
			}
			if off < 0 || off >= len(arr) {
				return qs, nil, fmt.Errorf("index %d out of range (0..%d)", off, len(arr))
			}
			out = append(out, arr[off])
		}
		return qs, out, nil
	})
}

type eachQuery struct{ Query }

func (q eachQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	return with(qs, v, func(a ast.Array) (*qstate, ast.Value, error) {
		var out ast.Array
		for i, elt := range a {
			_, v, err := q.Query.eval(qs, elt)
			if err != nil {
				return qs, nil, fmt.Errorf("index %d: %w", i, err)
			}
			out = append(out, v)
		}
		return qs, out, nil
	})
}

type lenQuery struct{}

func (lenQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	if t, ok := v.(interface {
		Len() int
	}); ok {
		return qs, ast.Int(t.Len()), nil
	}
	return qs, nil, fmt.Errorf("cannot take length of %T", v)
}

type recQuery struct{ Query }

func (q recQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	var out ast.Array

	type entry struct {
		s *qstate
		v ast.Value
	}
	stk := []entry{{qs, v}}
	for len(stk) != 0 {
		next := stk[len(stk)-1]
		stk = stk[:len(stk)-1]

		ns, r, err := q.Query.eval(next.s, next.v)
		if err == nil {
			if a, ok := r.(ast.Array); ok {
				out = append(out, a...)
			} else {
				out = append(out, r)
			}
		}

		// N.B. Push in reverse order, so we visit in lexical order.
		switch t := next.v.(type) {
		case ast.Object:
			for i := len(t) - 1; i >= 0; i-- {
				stk = append(stk, entry{ns, t[i].Value})
			}
		case ast.Array:
			for i := len(t) - 1; i >= 0; i-- {
				stk = append(stk, entry{ns, t[i]})
			}
		}
	}

	if len(out) == 0 {
		return qs, nil, errors.New("no matches")
	}
	return qs, out, nil
}

type delQuery struct{ name string }

func (d delQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	// As a special case, treat null as equivalent to an empty object.
	if v == ast.Null {
		return qs, v, nil
	}
	return with(qs, v, func(o ast.Object) (*qstate, ast.Value, error) {
		found := o.Find(d.name)
		if found == nil {
			return qs, o, nil
		}
		res := make(ast.Object, 0, o.Len()-1)
		for _, m := range o {
			if m == found {
				continue
			}
			res = append(res, m)
		}
		return qs, res, nil
	})
}

type setQuery struct {
	name string
	q    Query
}

func (s setQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	_, t, err := s.q.eval(qs, v)
	if err != nil {
		return qs, nil, err
	}
	if v == ast.Null {
		v = ast.Object{}
	}
	return with(qs, v, func(o ast.Object) (*qstate, ast.Value, error) {
		found := o.Find(s.name)
		if found == nil {
			return qs, append(o, &ast.Member{
				Key:   ast.String(s.name),
				Value: t,
			}), nil
		}
		out := make(ast.Object, len(o))
		for i, m := range o {
			if m == found {
				out[i] = &ast.Member{Key: ast.String(s.name), Value: t}
			} else {
				out[i] = m
			}
		}
		return qs, out, nil
	})
}

type constQuery struct{ ast.Value }

func (c constQuery) eval(qs *qstate, _ ast.Value) (*qstate, ast.Value, error) {
	return qs, c.Value, nil
}

type globQuery struct{}

func (globQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	switch t := v.(type) {
	case ast.Object:
		out := make(ast.Array, len(t))
		for i, m := range t {
			out[i] = m.Value
		}
		return qs, out, nil
	case ast.Array:
		return qs, t, nil
	default:
		return qs, nil, errors.New("no matching values")
	}
}

type keysQuery struct{}

func (keysQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	var out ast.Array
	if o, ok := v.(ast.Object); ok {
		for _, m := range o {
			out = append(out, m.Key)
		}
		return qs, out, nil
	} else if v == ast.Null {
		return qs, out, nil
	}

	return qs, nil, fmt.Errorf("cannot list keys of %T", v)
}

func isMarked(s string) (string, bool) {
	if s == "$" {
		return s, true
	} else if strings.HasPrefix(s, "$$") {
		return s[1:], false
	} else if strings.HasPrefix(s, "$") {
		return s[1:], true
	}
	return s, false
}

type getQuery struct{ name string }

func (q getQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	if w, ok := qs.lookup(q.name); ok {
		return qs, w, nil
	}
	return qs, nil, fmt.Errorf("parameter %q not found", q.name)
}

type asQuery struct {
	name string
	q    Query
}

func (q asQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	_, w, err := q.q.eval(qs, v)
	if err != nil {
		return qs, nil, err
	}
	return qs.bind(q.name, w), v, nil
}

type refQuery struct{ Query }

func (r refQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	_, w, err := r.Query.eval(qs, v)
	if err != nil {
		return qs, nil, err
	}
	switch t := w.(type) {
	case ast.Number:
		return nthQuery(int(t.Int())).eval(qs, v)
	case ast.Keyer:
		return objKey(t.Key()).eval(qs, v)
	}
	return qs, nil, fmt.Errorf("value %T is not a valid reference", w)
}

type selectQuery struct{ Query }

func (q selectQuery) eval(qs *qstate, v ast.Value) (*qstate, ast.Value, error) {
	return with(qs, v, func(a ast.Array) (*qstate, ast.Value, error) {
		var out ast.Array
		for _, elt := range a {
			if _, _, err := q.Query.eval(qs, elt); err == nil {
				out = append(out, elt)
			}
		}
		return qs, out, nil
	})
}

func with[T ast.Value](qs *qstate, v ast.Value, f func(T) (*qstate, ast.Value, error)) (*qstate, ast.Value, error) {
	if v, ok := v.(T); ok {
		return f(v)
	}
	var zero T
	return qs, nil, fmt.Errorf("got %T, want %T", v, zero)
}

type qstate struct {
	name  string
	value ast.Value
	up    *qstate
}

func (s *qstate) bind(name string, value ast.Value) *qstate {
	return &qstate{name: name, value: value, up: s}
}

func (s *qstate) lookup(name string) (ast.Value, bool) {
	for cur := s; cur != nil; cur = cur.up {
		if cur.name == name {
			return cur.value, true
		}
	}
	return nil, false
}
