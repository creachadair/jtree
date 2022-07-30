// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree_test

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/creachadair/jtree"
	"github.com/google/go-cmp/cmp"
)

func TestStream(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "."},
		{"   ", "."},

		{"true false null", `
Value true <true>
Value false <false>
Value null <null>
.`},

		{`0 5 -6.32 0.1e-2`, `
Value integer <0>
Value integer <5>
Value number <-6.32>
Value number <0.1e-2>
.`},

		{`"" "a b c" "a\tb" "a\u0020b"`, `
Value string <"">
Value string <"a b c">
Value string <"a\tb">
Value string <"a\u0020b">
.`},

		{`{}`, "BeginObject\nEndObject\n."},

		{`{"a":15}`, `
BeginObject
BeginMember <"a">
Value integer <15>
EndMember "}"
EndObject
.`},

		{`{"x":null, "y":[true]}`, `
BeginObject
BeginMember <"x">
Value null <null>
EndMember ","
BeginMember <"y">
BeginArray
Value true <true>
EndArray
EndMember "}"
EndObject
.`},

		{`[]`, "BeginArray\nEndArray\n."},
	}

	for _, test := range tests {
		st := jtree.NewStream(strings.NewReader(test.input))
		th := new(testHandler)
		if err := st.Parse(th); err != nil {
			t.Errorf("Parse failed: %v", err)
		}

		if diff := diffStrings(test.want, th.output()); diff != "" {
			t.Errorf("Input: %#q\nOutput: (-want, +got)\n%s", test.input, diff)
		}
	}
}

func TestStreamErrors(t *testing.T) {
	tests := []struct {
		input string
		want  string
		estr  string
	}{
		// Various kinds of unbalanced object bits.
		{`{`, `BeginObject`,
			`at 1:1: expected "}" or string, got error: EOF`},
		{`}`, ``, `at 1:0: unexpected "}"`},
		{`{false:1}`, `BeginObject`,
			`at 1:1: expected "}" or string, got false`},
		{`{"true":}`, `
BeginObject
BeginMember <"true">`,
			`at 1:8: unexpected "}"`},
		{`{"true":1,`, `
BeginObject
BeginMember <"true">
Value integer <1>
EndMember ","`,
			`at 1:10: expected string, got error: EOF`},

		// Unbalanced array bits.
		{`[`, `BeginArray`,
			`at 1:1: expected more input, got error: EOF`},
		{`]`, ``, `at 1:0: unexpected "]"`},
		{`[15,`, `
BeginArray
Value integer <15>`,
			`at 1:4: expected more input, got error: EOF`},
		{`[15,]`, `
BeginArray
Value integer <15>`,
			`at 1:4: unexpected "]"`},

		// Invalid values.
		{`1 2.0 forthright`, `
Value integer <1>
Value number <2.0>`,
			`at 1:6: invalid input`},
		{`"what did you`, ``,
			`at 1:0: invalid input`},
	}

	for _, test := range tests {
		st := jtree.NewStream(strings.NewReader(test.input))
		th := new(testHandler)
		err := st.Parse(th)
		if err == nil {
			t.Error("Parse did not report an error")
			continue
		}

		if diff := diffStrings(test.want, th.output()); diff != "" {
			t.Errorf("Input: %#q\nOutput: (-want, +got)\n%s", test.input, diff)
		}
		if diff := diffStrings(test.estr, err.Error()); diff != "" {
			t.Errorf("Input: %#q\nError: (-want, +got)\n%s", test.input, diff)
		}
	}
}

func TestParseOne(t *testing.T) {
	const input = `{ "love": true } [] "ok"`
	const want = `
BeginObject
BeginMember <"love">
Value true <true>
EndMember "}"
EndObject
---
BeginArray
EndArray
---
Value string <"ok">
---
.`
	th := new(testHandler)

	st := jtree.NewStream(strings.NewReader(input))
	for {
		err := st.ParseOne(th)
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("ParseOne failed: %v", err)
		}
		th.pr("---")
	}

	if diff := diffStrings(want, th.output()); diff != "" {
		t.Errorf("Input: %#q\nOutput: (-want, +got)\n%s", input, diff)
	}
}

func diffStrings(want, got string) string {
	return cmp.Diff(strings.Split(strings.TrimSpace(want), "\n"),
		strings.Split(strings.TrimSpace(got), "\n"))
}

type testHandler struct {
	buf bytes.Buffer
}

func (t *testHandler) pr(msg string, args ...any) {
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}
	fmt.Fprintf(&t.buf, msg, args...)
}

func (t *testHandler) output() string { return t.buf.String() }

func (t *testHandler) BeginObject(loc jtree.Anchor) error { t.pr("BeginObject"); return nil }
func (t *testHandler) EndObject(loc jtree.Anchor) error   { t.pr("EndObject"); return nil }
func (t *testHandler) BeginArray(loc jtree.Anchor) error  { t.pr("BeginArray"); return nil }
func (t *testHandler) EndArray(loc jtree.Anchor) error    { t.pr("EndArray"); return nil }
func (t *testHandler) EndOfInput(loc jtree.Anchor)        { t.pr(".") }

func (t *testHandler) BeginMember(loc jtree.Anchor) error {
	t.pr("BeginMember <%s>", string(loc.Text()))
	return nil
}

func (t *testHandler) EndMember(loc jtree.Anchor) error {
	t.pr("EndMember %s", loc.Token())
	return nil
}

func (t *testHandler) Value(loc jtree.Anchor) error {
	t.pr(`Value %s <%s>`, loc.Token(), string(loc.Text()))
	return nil
}

func (t *testHandler) SyntaxError(loc jtree.Anchor, err error) error {
	t.pr("SyntaxError %v", err)
	return nil
}
