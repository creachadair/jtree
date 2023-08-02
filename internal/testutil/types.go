// Package testutil defines support code for unit tests.
package testutil

import (
	"reflect"
	"strings"

	"github.com/creachadair/jtree/ast"
)

// RawNumberType is a value of the internal type used to represent raw numbers
// parsed from JSON source text.
var RawNumberType any

func init() {
	q, err := ast.ParseSingle(strings.NewReader("1"))
	if err != nil {
		panic(err)
	}
	RawNumberType = reflect.Zero(reflect.TypeOf(q)).Interface()
}
