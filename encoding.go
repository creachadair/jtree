// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"errors"

	"github.com/creachadair/jtree/internal/escape"

	"go4.org/mem"
)

// Quote encodes src as a JSON string value. The contents are escaped and
// double quotation marks are added.
func Quote(src string) string { return escape.Quote(mem.S(src)).StringCopy() }

// Unquote decodes a JSON string value.  Double quotation marks are removed,
// and escape sequences are replaced with their unescaped equivalents.
//
// Invalid escapes are replaced by the Unicode replacement rune. Unquote
// reports an error for an incomplete escape sequence.
func Unquote(src []byte) ([]byte, error) { return unquoteMem(mem.B(src)) }

// UnquoteString decodes a JSON string value.  Double quotation marks are
// removed, and escape sequences are replaced with their unescaped equivalents.
//
// Invalid escapes are replaced by the Unicode replacement rune. Unquote
// reports an error for an incomplete escape sequence.
func UnquoteString(src string) ([]byte, error) { return unquoteMem(mem.S(src)) }

var dquote = mem.S(`"`)

func unquoteMem(src mem.RO) ([]byte, error) {
	if src.Len() < 2 || !mem.HasPrefix(src, dquote) || !mem.HasSuffix(src, dquote) {
		return nil, errors.New("missing quotations")
	}
	return escape.Unquote(src.Slice(1, src.Len()-1))
}

// Interner is a deduplicating string interning map.
type Interner map[string]string

// Intern returns text as a string, ensuring that only one string is allocated
// for each unique text.
func (n Interner) Intern(text []byte) string {
	s, ok := n[string(text)] // N.B. lookup special-cased by the compiler
	if !ok {
		s = string(text)
		n[s] = s
	}
	return s
}

// ParseInt behaves as strconv.ParseInt, but does not copy its argument.
func ParseInt(text []byte, base, bitSize int) (int64, error) {
	return mem.ParseInt(mem.B(text), base, bitSize)
}

// ParseFloat behaves as strconv.ParseFloat, but does not copy its argument.
func ParseFloat(text []byte, bitSize int) (float64, error) {
	return mem.ParseFloat(mem.B(text), bitSize)
}
