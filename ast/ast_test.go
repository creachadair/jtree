// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package ast_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/google/go-cmp/cmp"
)

const testJSON = `{
  "list": [
    {
      "x": 1
    },
    {
      "x": 2
    }
  ],
  "y": {
    "hello": "there"
  },
  "o": [
    "hi",
    "yourself"
  ],
  "xyz": {
    "p": true,
    "d": true,
    "q": false
  }
}`

func TestPath(t *testing.T) {
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
		{"ArrayRange", []any{"o", 25}, v, true},
		{"ObjPath", []any{"xyz", "d"},
			v.(ast.Object).Find("xyz").Value.(ast.Object).Find("d").Value,
			false,
		},

		{"FuncArray", []any{"o", testPathFunc}, ast.ToValue(2), false},
		{"FuncObj", []any{"xyz", testPathFunc}, ast.ToValue(3), false},
		{"FuncWrong", []any{"xyz", "d", testPathFunc}, v, true},
	}
	opt := cmp.AllowUnexported(
		ast.Quoted{},
		ast.Number{},
	)
	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			got, err := ast.Path(v, tc.path...)
			if err != nil {
				if tc.fail {
					t.Logf("Got expected error: %v", err)
				} else {
					t.Fatalf("Path: unexpected error: %v", err)
				}
			}
			if diff := cmp.Diff(got, tc.want, opt); diff != "" {
				t.Errorf("Wrong result (-got, +want):\n%s", diff)
			} else if err == nil {
				t.Logf("Found %s OK", got.JSON())
			}
		})
	}
}

func testPathFunc(v ast.Value) (ast.Value, error) {
	if ln, ok := v.(interface{ Len() int }); ok {
		return ast.ToValue(ln.Len()), nil
	}
	return nil, errors.New("not a thing with length")
}
