// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc

import (
	"strings"

	"github.com/creachadair/jtree/ast"
)

// CleanComments combines and removes comment markers from the given comments,
// returning a slice of plain lines of text. Leading and trailing spaces are
// removed from the lines.
func CleanComments(coms ...string) []string {
	var out []string
	for _, com := range coms {
		_, text := classifyComment(com)
		lines := strings.Split(text, "\n")
		outdentCommentLines(lines)
		for _, line := range lines {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

// Decorate converts an ast.Value into an equivalent jwcc.Value.
func Decorate(v ast.Value) Value {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case ast.Object:
		o := &Object{Members: make([]*Member, len(t))}
		for i, m := range t {
			o.Members[i] = &Member{Key: m.Key, Value: Decorate(m.Value)}
		}
		return o
	case ast.Array:
		a := &Array{Values: make([]Value, len(t))}
		for i, v := range t {
			a.Values[i] = Decorate(v)
		}
		return a
	default:
		return &Datum{Value: v}
	}
}
