// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jtree

import (
	"github.com/creachadair/jtree/internal/escape"

	"go4.org/mem"
)

// Quote encodes a string to escape characters for inclusion in a JSON string.
func Quote(src string) []byte {
	return escape.Quote(mem.S(src))
}

// Unquote decodes a byte slice containing the JSON encoding of a string. The
// input must have the enclosing double quotation marks already removed.
//
// Escape sequences are replaced with their unescaped equivalents. Invalid
// escapes are replaced by the Unicode replacement rune. Unquote reports an
// error for an incomplete escape sequence.
func Unquote(src []byte) ([]byte, error) {
	return escape.Unquote(mem.B(src))
}
