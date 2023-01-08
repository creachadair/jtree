package query_test

import (
	"fmt"
	"log"
	"strings"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/query"
)

func mustParseOne(s string) ast.Value {
	vals, err := ast.Parse(strings.NewReader(s))
	if err != nil {
		log.Fatalf("Parse: %v", err)
	} else if len(vals) != 1 {
		log.Fatalf("Got %d values, want 1", len(vals))
	}
	return vals[0]
}

func Example_small() {
	root := mustParseOne(`[{"a": 1, "b": 2}, {"c": {"d": true}, "e": false}]`)
	v, err := query.Eval(root, query.Path(1, "c", "d"))
	if err != nil {
		log.Fatalf("Eval: %v", err)
	}
	fmt.Println(v.JSON())
	// Output:
	// true
}

func Example_medium() {
	root := mustParseOne(`
{
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

	v, err := query.Eval(root, query.Object{
		"name": query.Path("plaintiff"),
		"act": query.Array{
			query.Path("complaint", "defendant"),
			query.Path("complaint", "action"),
			query.Value("my"),
			query.Path("relatedPersons", "Individual 1", "id"),
		},
		"req": query.Path("requestedRelief", 0),
	})
	if err != nil {
		log.Fatalf("Eval: %v", err)
	}
	obj := v.(ast.Object)
	fmt.Printf("Hello, my name is: %s\n", obj.Find("name").Value)
	fmt.Println(obj.Find("act").Value.JSON())
	fmt.Printf("Prepare to %s", obj.Find("req").Value)
	// Output:
	// Hello, my name is: Inigo Montoya
	// ["you","killed","my","father"]
	// Prepare to die
}
