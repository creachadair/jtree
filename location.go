package jtree

// A Span describes a contiguous span of a source input.
type Span struct {
	Pos int // the start offset, 0-based
	End int // the end offset, 0-based (noninclusive)
}

// A LineCol describes the line number and column offset of a location in
// source text.
type LineCol struct {
	Line   int // line number, 1-based
	Column int // byte offset of column in line, 0-based
}

// A Location describes the complete location of a range of source text,
// including line and column offsets.
type Location struct {
	Span
	First, Last LineCol
}
