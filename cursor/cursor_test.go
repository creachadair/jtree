// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package cursor_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/cursor"
	"github.com/creachadair/jtree/internal/testutil"
	"github.com/creachadair/jtree/jwcc"
	"github.com/google/go-cmp/cmp"

	_ "embed"
)

//go:embed testdata/cursor.json
var testJSON string

//go:embed testdata/cursor.jwcc
var testJWCC string

func TestCursor(t *testing.T) {
	v, err := ast.ParseSingle(strings.NewReader(testJSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		name string
		path []any
		want ast.Value
		fail bool
	}{
		{"NilInput", nil, v, false},
		{"NoMatch", []any{"nonesuch"}, v, true},
		{"WrongType", []any{11}, v, true},

		{"ArrayPos", []any{"list", 1},
			v.(ast.Object).Find("list").Value.(ast.Array)[1],
			false,
		},
		{"ArrayNeg", []any{"list", -1},
			v.(ast.Object).Find("list").Value.(ast.Array)[1],
			false,
		},
		{"ArrayRange", []any{"o", 25},
			v.(ast.Object).Find("o").Value,
			true,
		},
		{"ObjPath", []any{"xyz", "d"},
			v.(ast.Object).Find("xyz").Value.(ast.Object).Find("d"),
			false,
		},
		{"ObjPathTail", []any{"xyz", "d", nil},
			ast.ToValue(true),
			false,
		},

		{"FuncMatch", []any{"xyz", ast.TextEqualFold("D"), nil},
			ast.ToValue(true),
			false,
		},
		{"FuncNoMatch", []any{ast.TextEqual("?")}, v, true},
		{"FuncArray", []any{"o", testPathFunc}, ast.ToValue(2), false},
		{"FuncObj", []any{"xyz", testPathFunc}, ast.ToValue(3), false},
		{"FuncWrong", []any{"xyz", "d", testPathFunc},
			v.(ast.Object).
				FindKey(ast.TextEqual("xyz")).Value.(ast.Object).
				FindKey(ast.TextEqual("d")).Value,
			true,
		},
	}
	opt := cmp.AllowUnexported(ast.String("").Quote(), testutil.RawNumberType)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := cursor.New(v).Down(tc.path...)
			err := c.Err()
			if err != nil {
				if tc.fail {
					t.Logf("Got expected error: %v", err)
				} else {
					t.Fatalf("Down %+v: unexpected error: %v", tc.path, err)
				}
			}
			got := c.Value()
			if diff := cmp.Diff(got, tc.want, opt); diff != "" {
				t.Errorf("Down %+v: wrong result (-got, +want):\n%s", tc.path, diff)
			} else if err == nil {
				t.Logf("Found %s OK", got.JSON())
				t.Log("Path:")
				for i, p := range c.Path() {
					t.Logf("%d: %+v", i+1, p)
				}
			}
		})
	}
}

func TestCursorJWCC(t *testing.T) {
	doc, err := jwcc.Parse(strings.NewReader(testJWCC))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	tests := []struct {
		name string
		path []any
		want jwcc.Value
		fail bool
	}{
		{"NilInput", nil, doc.Value, false},
		{"NoMatch", []any{"nonesuch"}, doc.Value, true},
		{"WrongType", []any{11}, doc.Value, true},

		{"ArrayPos", []any{"list", 1},
			doc.Value.(*jwcc.Object).Find("list").Value.(*jwcc.Array).Values[1],
			false,
		},
		{"ArrayNeg", []any{"list", -1},
			doc.Value.(*jwcc.Object).Find("list").Value.(*jwcc.Array).Values[1],
			false,
		},
		{"ArrayRange", []any{"o", 25},
			doc.Value.(*jwcc.Object).Find("o").Value,
			true,
		},
		{"ObjPath", []any{"xyz", "d"},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d"),
			false,
		},
		{"ObjPathTail", []any{"xyz", "d", nil},
			doc.Value.(*jwcc.Object).Find("xyz").Value.(*jwcc.Object).Find("d").Value,
			false,
		},

		{"FuncArray", []any{"o", testPathFunc}, jwcc.ToValue(2), false},
		{"FuncObj", []any{"xyz", testPathFunc}, jwcc.ToValue(3), false},
		{"FuncWrong", []any{"xyz", "d", testPathFunc},
			doc.Value.(*jwcc.Object).
				FindKey(ast.TextEqual("xyz")).Value.(*jwcc.Object).
				FindKey(ast.TextEqual("d")).Value,
			true,
		},
	}
	opt := cmp.AllowUnexported(
		ast.String("").Quote(),
		testutil.RawNumberType,
		jwcc.Array{},
		jwcc.Comments{},
		jwcc.Datum{},
		jwcc.Member{},
		jwcc.Object{},
	)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := cursor.New(doc.Value).Down(tc.path...)
			err := c.Err()
			if err != nil {
				if tc.fail {
					t.Logf("Got expected error: %v", err)
				} else {
					t.Fatalf("Down %+v: unexpected error: %v", tc.path, err)
				}
			}
			got := c.Value()
			if diff := cmp.Diff(got, tc.want, opt); diff != "" {
				t.Errorf("Down %+v: wrong result (-got, +want):\n%s", tc.path, diff)
			} else if err == nil {
				t.Logf("Found %s OK", got.JSON())
				t.Log("Path:")
				for i, p := range c.Path() {
					t.Logf("%d: %+v", i+1, p)
				}
			}
		})
	}
}

func testPathFunc(v ast.Value) (ast.Value, error) {
	switch t := v.(type) {
	case ast.Array:
		return ast.ToValue(len(t)), nil
	case *jwcc.Array:
		return jwcc.ToValue(len(t.Values)), nil
	case ast.Object:
		return ast.ToValue(len(t)), nil
	case *jwcc.Object:
		return jwcc.ToValue(len(t.Members)), nil
	default:
		return nil, errors.New("not a thing with length")
	}
}
