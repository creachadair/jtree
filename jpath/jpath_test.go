package jpath_test

import (
	"testing"

	"github.com/creachadair/jtree/jpath"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"$.store.book[*]..author"},
		{"$..author"},
		{"$.store.*"},
		{"$.store..price"},
		{"$..book[2]"},
		{"$..book[(@.length-1)]"},
		{"$..book[-1:]"},
		{"$..book[0,1]"},
		{"$..book[:2]"},
		{"$..book[?(@.isbn)]"},
		{"$..book[?(@price<10)]"},
		{"$..*"},
		{"$['apple sauce'].pearPlum..'cherry apple'"},
		{"$[a][1:3][b]['c d e']"},
	}
	for _, test := range tests {
		e, err := jpath.Parse(test.input)
		if err != nil {
			t.Errorf("Parse %q: %v", test.input, err)
			continue
		}

		want := test.input
		if got := e.String(); got != want {
			t.Errorf("Parse %q:\n got %q\nwant %q", test.input, got, want)
		}
	}
}
