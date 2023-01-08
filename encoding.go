// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
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
func Unquote(src string) ([]byte, error) {
	content := mem.TrimPrefix(mem.TrimSuffix(mem.S(src), mem.S(`"`)), mem.S(`"`))
	return escape.Unquote(content)
}
