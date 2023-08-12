// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package tq_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/tq"
)

func mustParseString(s string) ast.Value {
	val, err := ast.ParseSingle(strings.NewReader(s))
	if err != nil {
		log.Fatalf("Parse: %v", err)
	}
	return val
}

func Example_simple() {
	root := mustParseString(`[{"a": 1, "b": 2}, {"c": {"d": true}, "e": false}]`)

	v, err := tq.Eval[ast.Bool](root, tq.Path(1, "c", "d"))

	if err != nil {
		log.Fatalf("Eval: %v", err)
	}
	fmt.Println(v.JSON())
	// Output:
	// true
}

func Example_small() {
	root := mustParseString(`[{"a": 1, "b": 2}, {"c": {"d": true}, "e": false}]`)

	v, err := tq.Eval[ast.Bool](root, tq.Path(
		tq.As("@", 1, "c"), "$@", "d",
	))
	if err != nil {
		log.Fatalf("Eval: %v", err)
	}
	fmt.Println(v.JSON())
	// Output:
	// true
}

func Example_medium() {
	root := mustParseString(`{
  "plaintiff": "Inigo Montoya",
  "complaint": {
     "defendant": "you",
     "action": "killed",
     "target": "Individual 1"
  },
  "requestedRelief": ["die", "pay punitive damages", "pay attorney fees"],
  "relatedPersons": {
    "Individual 1": {"id": "father", "rel": "plaintiff"}
  }
}`)

	obj, err := tq.Eval[ast.Object](root, tq.Path(
		// Bind c to the "complaint" object.
		tq.As("c", "complaint"),

		// Bindings can refer back to previous bindings in the frame.
		tq.As("t", "$c", "target"),

		// Use tq.Ref to use a query result as part of another query.
		tq.As("@", "relatedPersons", tq.Ref("$t"), "id"),

		// Construct an object with field values pulled from other places.
		tq.Object{
			"name": tq.Path("plaintiff"),

			"act": tq.Array{
				tq.Path("$c", "defendant"),
				tq.Path("$c", "action"),
				tq.Value("my"), // a constant value
				tq.Get("@"),
			},

			"req": tq.Path("requestedRelief", 0),
		},
	))
	if err != nil {
		log.Fatalf("Eval: %v", err)
	}
	fmt.Printf("Hello, my name is: %s\n", obj.Find("name").Value)
	fmt.Println(obj.Find("act").Value.JSON())
	fmt.Printf("Prepare to %s", obj.Find("req").Value)
	// Output:
	// Hello, my name is: Inigo Montoya
	// ["you","killed","my","father"]
	// Prepare to die
}
