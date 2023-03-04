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
