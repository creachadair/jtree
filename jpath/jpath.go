// Package jpath implements a minimal JSONPath expression parser.
package jpath

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

/*
Grammar:

  expr = root steps
  root = "$"
 steps = step [steps]
  step = "." name
  step = ".." name
  step = "[" value "]"
  step = "[" slice "]"
  name = WORD
  name = "'" QTEXT "'"
  name = "*"
 value = name
 value = INDEX
 value = script
 value = filter
 slice = INDEX ":" INDEX
script = "(" TEXT ")"
filter = "?(" TEXT ")"

  WORD = RE `\w+`
 QTEXT = RE `([^']|\\')*`
 INDEX = RE `-?\d+`
  TEXT = { all text with nested parentheses }

Source:
  https://www.ietf.org/archive/id/draft-goessner-dispatch-jsonpath-00.html
*/

// An Expr is a parsed JSONPath expression.
type Expr []Step

// Parse parses s as a JSONPath expression.
func Parse(s string) (Expr, error) {
	st, _, err := parseExpr(s)
	if err != nil {
		return Expr{}, err
	}
	return st, nil
}

func (e Expr) String() string {
	var buf strings.Builder
	buf.WriteString("$")
	for _, s := range e {
		switch s.Op {
		case Member, Recur:
			if s.Arg2 == "qname" {
				fmt.Fprintf(&buf, "%s'%s'", s.Op, s.Arg1)
			} else {
				fmt.Fprint(&buf, s.Op, s.Arg1)
			}

		case Slice:
			fmt.Fprintf(&buf, "[%s:%s]", s.Arg1, s.Arg2)

		case Script:
			fmt.Fprintf(&buf, "[(%s)]", s.Arg1)

		case Filter:
			fmt.Fprintf(&buf, "[?(%s)]", s.Arg1)

		default:
			if s.Op == QName {
				fmt.Fprintf(&buf, "['%s']", s.Arg1)
			} else {
				fmt.Fprintf(&buf, "[%s]", s.Arg1)
			}
		}
	}
	return buf.String()
}

func parseExpr(s string) ([]Step, string, error) {
	t, ok := strings.CutPrefix(s, "$")
	if !ok {
		return nil, s, errors.New("missing root marker")
	}
	return parseSteps(t)
}

func parseSteps(s string) (steps []Step, rest string, _ error) {
	for s != "" {
		step, rest, err := parseStep(s)
		if err != nil {
			return nil, s, err
		}
		steps = append(steps, step)
		s = rest
	}
	return steps, s, nil
}

func parseStep(s string) (_ Step, rest string, _ error) {
	if t, ok := strings.CutPrefix(s, ".."); ok {
		kind, name, u, err := parseName(t)
		if err != nil {
			return Step{}, s, fmt.Errorf("invalid ..name: %w", err)
		}
		return Step{Op: Recur, Arg1: name, Arg2: kind.String()}, u, nil
	}
	if t, ok := strings.CutPrefix(s, "."); ok {
		kind, name, u, err := parseName(t)
		if err != nil {
			return Step{}, s, fmt.Errorf("invalid .name: %w", err)
		}
		return Step{Op: Member, Arg1: name, Arg2: kind.String()}, u, nil
	}
	if t, ok := strings.CutPrefix(s, "["); ok {
		kind, val, u, err := parseValue(t)
		if err != nil {
			return Step{}, t, err
		}
		out := Step{Op: kind, Arg1: val}
		if out.Op == Slice {
			arg2, rest, err := parseIndex(u)
			if err == nil {
				out.Arg2 = arg2
				u = rest
			} else if out.Arg1 == "" {
				return Step{}, u, errors.New("invalid slice")
			}
		}
		u, ok := strings.CutPrefix(u, "]")
		if !ok {
			return Step{}, u, errors.New("missing close bracket")
		}
		return out, u, nil
	}
	return Step{}, s, errors.New("invalid path step")
}

func parseName(s string) (kind Op, name, rest string, _ error) {
	if t, ok := strings.CutPrefix(s, "*"); ok {
		return Wildcard, "*", t, nil
	}
	if m := wordRE.FindStringSubmatch(s); m != nil {
		return Name, m[1], s[len(m[0]):], nil
	}
	if m := quoteRE.FindStringSubmatch(s); m != nil {
		return QName, m[1], s[len(m[0]):], nil
	}
	return Invalid, "", s, errors.New("invalid name")
}

func parseIndex(s string) (text, rest string, _ error) {
	if m := indexRE.FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):], nil
	}
	return "", "", errors.New("invalid index")
}

func parseValue(s string) (kind Op, value, rest string, _ error) {
	if t, ok := strings.CutPrefix(s, "?("); ok {
		text, rest, err := parseScript(t)
		return Filter, text, rest, err
	}
	if t, ok := strings.CutPrefix(s, "("); ok {
		text, rest, err := parseScript(t)
		return Script, text, rest, err
	}
	if text, rest, err := parseIndex(s); err == nil {
		if u, ok := strings.CutPrefix(rest, ":"); ok {
			return Slice, text, u, nil
		}
		return Index, text, rest, nil
	}
	if u, ok := strings.CutPrefix(s, ":"); ok {
		return Slice, "", u, nil
	}
	if kind, text, rest, err := parseName(s); err == nil {
		return kind, text, rest, nil
	}
	return Invalid, "", s, fmt.Errorf("invalid value: %q", s)
}

func parseScript(s string) (text, rest string, _ error) {
	i, np := 0, 1
	for i < len(s) {
		if s[i] == ')' {
			np--
			if np == 0 {
				break
			}
		} else if s[i] == '(' {
			np++
		}
		i++
	}
	if np > 0 {
		return "", s, errors.New("unbalanced parentheses")
	}
	return s[:i], s[i+1:], nil
}

var (
	wordRE  = regexp.MustCompile(`^(\w+)`)
	indexRE = regexp.MustCompile(`^(-?\d+(?:,-?\d+)*)`)
	quoteRE = regexp.MustCompile(`^'([^\']*)'`)
)

// An Op is a path operator.
type Op byte

const (
	Invalid  Op = iota // invalid operator
	Member             // member lookup (.)
	Index              // array index lookup
	Slice              // array slice
	Wildcard           // wildcard expansion (*)
	Name               // unquoted name expansion
	QName              // quoted name expansion
	Recur              // recur operator
	Filter             // filter operator
	Script             // script operator
)

var opText = map[Op]string{
	Invalid:  "invalid",
	Member:   ".",
	Index:    "index",
	Slice:    "slice",
	Wildcard: "*",
	Name:     "name",
	QName:    "qname",
	Recur:    "..",
	Filter:   "?(...)",
	Script:   "(...)",
}

func (o Op) String() string {
	if s, ok := opText[o]; ok {
		return s
	}
	return opText[Invalid]
}

// A Step is a single step of a JSONPath expression.
type Step struct {
	Op   Op
	Arg1 string
	Arg2 string
}
