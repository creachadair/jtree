// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package cursor_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/ast/cursor"
	"github.com/google/go-cmp/cmp"

	_ "embed"
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

		{"FuncArray", []any{"o", testPathFunc}, ast.ToValue(2), false},
		{"FuncObj", []any{"xyz", testPathFunc}, ast.ToValue(3), false},
		{"FuncWrong", []any{"xyz", "d", testPathFunc},
			v.(ast.Object).Find("xyz").Value.(ast.Object).Find("d").Value,
			true,
		},
	}
	opt := cmp.AllowUnexported(ast.Quoted{}, ast.Number{})
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
			}
		})
	}
}

func testPathFunc(v ast.Value) (ast.Value, error) {
	switch t := v.(type) {
	case ast.Array:
		return ast.ToValue(len(t)), nil
	case ast.Object:
		return ast.ToValue(len(t)), nil
	default:
		return nil, errors.New("not a thing with length")
	}
}
