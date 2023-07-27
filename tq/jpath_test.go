package tq_test

import (
	"testing"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/jtree/tq"
)

// Examples from https://www.ietf.org/archive/id/draft-goessner-dispatch-jsonpath-00.html
// Double-checked against the evaluator from https://jsonpath.com/.
//
// To generate more examples, paste testdata/jsonpath.json into the evaluator,
// evaluate the expression, and copy the desired output out of the right pane.
// For the tests I used jq -c to remove whitespace from the want values.
func TestJSONPathInputs(t *testing.T) {
	val := mustParseFile(t, "../testdata/jsonpath.json")

	tests := []struct {
		name  string
		orig  string // original query (for documentation, not used here)
		query tq.Query
		want  string // JSON
	}{
		{
			"AllAuthors1", "$.store.book[*].author",
			tq.Path("store", "book", tq.Each("author")),

			`["Nigel Rees","Evelyn Waugh","Herman Melville","J. R. R. Tolkien"]`,
		},
		{
			"AllAuthors2", "$..author",
			tq.Recur("author"),

			`["Nigel Rees","Evelyn Waugh","Herman Melville","J. R. R. Tolkien"]`,
		},
		{
			"AllPrices", "$.store..price",
			tq.Path("store", tq.Recur("price")),

			`[8.95,12.99,8.99,22.99,19.95]`,
		},
		{
			"WholeStore", "$.store.*",
			tq.Path("store", tq.Glob()),

			`[[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}],{"color":"red","price":19.95}]`,
		},
		{
			"Book2", "$..book[2]",
			tq.Recur("book", 2),

			`[{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99}]`,
		},
		{
			"LastBook1", "$..book[-1:]",
			tq.Recur("book", tq.Slice(-1, 0)),

			`[{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}]`,
		},
		{
			"LastBook2", "$..book[(@.length-1)]",
			tq.Recur("book", tq.Ref(tq.Map(func(v ast.Array) (ast.Int, error) {
				return ast.Int(v.Len() - 1), nil
			}))),

			`[{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}]`,
		},
		{
			"FirstTwoBooks1", "$..book[0,1]",
			tq.Recur("book", tq.Pick(0, 1)),

			`[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99}]`,
		},
		{
			"FirstTwoBooks2", "$..book[:2]",
			tq.Recur("book", tq.Slice(0, 2)),

			`[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99}]`,
		},
		{
			"FilterISBN", "$..book[?(@.isbn)]",
			tq.Recur("book", tq.Select(tq.Match(func(v ast.Object) bool {
				return v.Find("isbn") != nil
			}))),

			`[{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}]`,
		},
		{
			"CheapBooks", "$..book[?(@.price<10)]",
			tq.Recur("book", tq.Select(tq.Match(func(v ast.Object) bool {
				return v.Find("price").Value.(ast.Numeric).Float() < 10
			}))),

			`[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99}]`,
		},
		{
			"DevilSplat", "$..*",
			tq.Recur(tq.Glob()),

			`[{"book":[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}],"bicycle":{"color":"red","price":19.95}},[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99}],{"color":"red","price":19.95},{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author":"J. R. R. Tolkien","title":"The Lord of the Rings","isbn":"0-395-19395-8","price":22.99},"reference","Nigel Rees","Sayings of the Century",8.95,"fiction","Evelyn Waugh","Sword of Honour",12.99,"fiction","Herman Melville","Moby Dick","0-553-21311-3",8.99,"fiction","J. R. R. Tolkien","The Lord of the Rings","0-395-19395-8",22.99,"red",19.95]`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v, err := tq.Eval(val, test.query)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			got := v.JSON()
			if got != test.want {
				t.Errorf("Result:\n got %#q,\nwant %#q", got, test.want)
			}
		})
	}
}
