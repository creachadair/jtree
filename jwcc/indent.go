// Copyright (C) 2023 Michael J. Fromberger. All Rights Reserved.

package jwcc

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/creachadair/jtree/ast"
	"github.com/creachadair/mds/value"
)

// A Formatter carries the settings for pretty-printing JWCC values.
// A zero value is ready for use with default settings.
type Formatter struct {
	// If positive, the maximum number of array elements that may be rendered on
	// a single line. If this is zero or negative and MaxInlineArrayLength is
	// positive, there is no fixed limit; otherwise the default limit is 3.
	MaxInlineArrayElements int

	// If positive, the maximum length of a single line containing inline array
	// elements. If this is zero or negative, there is no fixed limit.
	MaxInlineArrayLength int

	// If non-empty, the string used as the increment of indentation.
	// If empty, the default is "\x20\x20", that is, two ASCII spaces.
	Indent string
}

func (f Formatter) indent() string { return cmp.Or(f.Indent, "  ") }

func (f Formatter) maxInlineArrayElements() int {
	if n := f.MaxInlineArrayElements; n > 0 {
		return n
	} else if f.MaxInlineArrayLength > 0 {
		return 0 // means: no limit
	}
	return 3
}

func (f Formatter) maxInlineArrayLength() int { return f.MaxInlineArrayLength }

// Format renders a pretty-printed representation of v to w with default
// settings.
func Format(w io.Writer, v Value) error {
	var f Formatter
	return f.Format(w, v)
}

// FormatToString formats v to a string with default settings.
// In case of error in formatting, it returns an empty string.
func FormatToString(v Value) string { return Formatter{}.FormatToString(v) }

// Format renders a pretty-printed representation of v to w using the settings
// from f.
func (f Formatter) Format(w io.Writer, v Value) error {
	tw := tabwriter.NewWriter(w, 4, 4, 1, ' ', 0)
	f.formatValue(tw, v, "", "", true)
	return tw.Flush()
}

// FormatToString formats v to a string with the settings in f.  In case of
// errors in formatting, it returns an empty string.
func (f Formatter) FormatToString(v Value) string {
	var buf bytes.Buffer
	if f.Format(&buf, v) != nil {
		return ""
	}
	return buf.String()
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

// formatValue writes a representation of v to w indented by indent.
// If lineCom is true, it renders a line-ending comment for v, if present.
func (f Formatter) formatValue(w writeFlusher, v Value, init, indent string, lineCom bool) {
	com := v.Comments()
	f.indentComments(w, com.Before, indent, false)
	switch t := v.(type) {
	case *Array:
		f.formatArray(w, t, init, indent)
	case *Datum:
		fmt.Fprint(w, init, t.JSON())
	case *Document:
		f.formatValue(w, t.Value, init, indent, lineCom)
		if ec := t.Comments().End; len(ec) != 0 {
			io.WriteString(w, "\n")
			f.indentComments(w, ec, indent, false)
		}
	case *Object:
		f.formatObject(w, t, init, indent)
	default:
		panic(fmt.Sprintf("unknown value type %T", v))
	}
	if lineCom && com.Line != "" {
		fmt.Fprint(w, indentComment(com.Line, "\t"), "\n")
	}
}

func (f Formatter) formatArray(w writeFlusher, a *Array, init, indent string) bool {
	if f.isBoring(a, len(init)) {
		fmt.Fprint(w, init, "[")
		for i, v := range a.Values {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			// We know there can be no line comment, since the array is boring.
			f.formatValue(w, v, "", "", false)
		}
		io.WriteString(w, "]")
		return true
	}

	// Before comments were already written.
	fmt.Fprint(w, init, "[\n")
	adent := indent + f.indent()
	for _, v := range a.Values {
		f.formatValue(w, v, adent, adent, false)

		// Render a line comment (if there is one) outside the comma.
		if ln := v.Comments().Line; ln != "" {
			fmt.Fprint(w, ",", indentComment(ln, "\t"), "\n")
		} else {
			fmt.Fprint(w, ",\n")
		}
	}

	// Insert trailer comments.
	if ec := a.Comments().End; len(ec) != 0 {
		if len(a.Values) != 0 {
			io.WriteString(w, "\n")
		}
		f.indentComments(w, ec, adent, false)
	}
	w.Flush()
	fmt.Fprint(w, indent, "]")
	return false
}

func (f Formatter) formatObject(w writeFlusher, o *Object, init, indent string) bool {
	if f.isBoring(o, len(init)) {
		fmt.Fprint(w, init, "{")
		for i, m := range o.Members {
			if i > 0 {
				io.WriteString(w, ", ")
			}
			fmt.Fprint(w, m.Key.JSON(), ": ")
			// We know there can be no line comment, since the object is boring.
			f.formatValue(w, m.Value, "", "", false)
		}
		io.WriteString(w, "}")
		return true
	}

	// Before comments were already written.
	fmt.Fprint(w, init, "{\n")
	mdent := indent + f.indent()
	prevBoring, curBoring := true, true
	for i, m := range o.Members {
		// Leave extra space before the next member if either it or its
		// predecessor was non-boring.
		prevBoring, curBoring = curBoring, f.isBoring(m, len(mdent))

		if i != 0 && !(prevBoring && curBoring) {
			io.WriteString(w, "\n")
		}

		f.indentComments(w, m.Comments().Before, mdent, false)
		fmt.Fprint(w, mdent, m.Key.JSON(), f.objSep(m.Value, len(mdent)))

		if len(m.Value.Comments().Before) == 0 {
			f.formatValue(w, m.Value, "", mdent, false)
		} else {
			// The value has some comments that need rendering, bump it in one more level.
			vdent := mdent + f.indent()
			io.WriteString(w, "\n")
			f.formatValue(w, m.Value, vdent, vdent, false)
		}

		// Render a line comment (if there is one) outside the comma.
		//
		// Note that the member OR the value may have a comment.
		// The parser will not attach a line comment to the member, but a
		// constructed member may have one. If both are set, prefer the value's.
		if ln := m.Value.Comments().Line; ln != "" {
			fmt.Fprint(w, ",", indentComment(ln, "\t"), "\n") // value's comment
		} else if ln := m.Comments().Line; ln != "" {
			fmt.Fprint(w, ",", indentComment(ln, "\t"), "\n") // member's comment
		} else {
			fmt.Fprint(w, ",\n")
		}
		if ec := m.Comments().End; len(ec) != 0 {
			f.indentComments(w, ec, mdent, false)
		}
	}

	// Insert trailer comments.
	if ec := o.Comments().End; len(ec) != 0 {
		if len(o.Members) != 0 {
			io.WriteString(w, "\n")
		}
		f.indentComments(w, ec, mdent, false)
	}
	w.Flush()
	fmt.Fprint(w, indent, "}")
	return false
}

// objSep returns a key-value separator for the given value.
// Boring values get indented so they line up in columns;
// non-boring values are stapled directly to the key.
func (f Formatter) objSep(v Value, ilen int) string {
	if f.isBoring(v, ilen) {
		return ":\t"
	}
	return ": "
}

// canInlineComment reports whether comments ss have simple enough structure
// that they can be rendered inline.
//
// TODO(creachadair): We might not need this anymore.
func (Formatter) canInlineComment(ss []string) bool {
	if len(ss) == 1 {
		return strings.HasPrefix(ss[0], "/*") && !strings.Contains(ss[0], "\n")
	}
	return false
}

// isBoring reports whether v has a simple enough structure that it can be
// rendered on one line, assuming the target line begins indented by ilen
// bytes.
func (f Formatter) isBoring(v Value, ilen int) bool {
	com := v.Comments()
	switch t := v.(type) {
	case *Array:
		if len(com.Before) != 0 || len(com.End) != 0 {
			return false
		}
		if slices.ContainsFunc(t.Values, func(elt Value) bool {
			return !f.isBoring(elt, ilen) || len(elt.Comments().Before) != 0
		}) {
			return false // something in there is not boring enough
		}
		if m := f.maxInlineArrayElements(); m > 0 && len(t.Values) > m {
			return false // too many items
		}
		if m := f.maxInlineArrayLength(); m > 0 {
			// Note: Include ilen, the indentation prior to the array.
			if elen := f.estimatedLength(t); elen+ilen+len(com.Line) > m {
				return false // too long for a line
			}
		}
		return true
	case *Datum:
		return t.Comments().IsEmpty()
	case *Member:
		return len(com.Before) == 0 && len(com.End) == 0 && f.isBoring(t.Value, ilen)
	case *Object:
		if len(com.Before) != 0 || len(com.End) != 0 {
			return false
		}
		if len(t.Members) == 1 {
			m0 := t.Members[0]
			return m0.Comments().IsEmpty() && m0.Value.Comments().IsEmpty() && f.isBoring(m0.Value, ilen)
		}
		return len(t.Members) == 0
	default:
		return false
	}
}

func (f Formatter) indentComments(w writeFlusher, ss []string, indent string, inlineOK bool) {
	if inlineOK && f.canInlineComment(ss) {
		fmt.Fprint(w, indentComment(ss[0], indent), " ")
		return
	}
	for _, s := range ss {
		if s == "" {
			io.WriteString(w, "\n")
			continue
		}
		fmt.Fprint(w, indentComment(s, indent), "\n")
	}
}

// estimatedLength reports an estimate of how long v would be if formatted in a
// single line of text, ignoring comments.  The estimate may be somewhat higher
// or lower than the real value.
func (f Formatter) estimatedLength(v Value) int {
	switch v := v.(type) {
	case *Array:
		var elen int
		if len(v.Values) != 0 {
			for _, elt := range v.Values {
				elen += f.estimatedLength(elt)
			}
			elen += 2 * (len(v.Values) - 1) // for the ", " between elements
		}
		return elen + 2 // +2 for "[" and "]"

	case *Object:
		var elen int
		if len(v.Members) != 0 {
			for _, mem := range v.Members {
				elen += f.estimatedLength(mem)
			}
			elen += 2 * (len(v.Members) - 1) // for the ", " between members
		}
		return elen + 2 // +2 for "{" and "}"

	case *Member:
		return len(v.Key.JSON()) + len(": ") + f.estimatedLength(v.Value)

	case *Datum:
		var nbuf [32]byte
		switch t := v.Value.(type) {
		case ast.Text:
			// Don't swizzle quotations here, this is close enough.
			// If it's from the parser, it's accurate; otherwise it is short by
			// the quotation marks and whatever escapes might be needed.
			// That could be a lot if it's a weird string, but that is uncommon.
			return len(t.Spelling())
		case ast.Bool:
			return value.Cond(t, len("true"), len("false"))
		case ast.Int:
			buf := strconv.AppendInt(nbuf[:0], int64(t), 10)
			return len(buf)
		case ast.Float:
			buf := strconv.AppendFloat(nbuf[:0], float64(t), 'g', -1, 64)
			return len(buf)
		case ast.Number:
			return len(t.JSON()) // this should be a rawNumber, so already formatted
		default:
			if t == ast.Null {
				return len("null")
			}
		}
		// fall through in case of something unexpected

	case *Document:
		return f.estimatedLength(v.Value)
	}
	return 0 // shouldn't happen, but fail gracefully
}

// indentComment realigns comment text from s and indents it by indent.
func indentComment(s, indent string) string {
	tag, text := classifyComment(s)
	var lines []string

	if strings.Count(text, "\n") == 0 {
		// The comment is just one line and is already trimmed.
		switch tag {
		case "/*":
			return indent + "/* " + text + " */"
		case "//":
			return indent + "//" + text
		default:
			return indent + "// " + text
		}
	}

	// The comment has multiple lines, and lines after the first are possibly
	// indented.
	lines = strings.Split(text, "\n")
	outdentCommentLines(lines)

	// Apply the indent and (if necessary) comment markers.
	all := make([]string, 0, len(lines)+2)
	if tag == "/*" {
		all = append(all, indent+"/*")
	}
	for _, line := range lines {
		switch tag {
		case "/*":
			all = append(all, indent+" "+line)
		case "//":
			all = append(all, indent+"//"+line)
		default:
			all = append(all, indent+"// "+line)
		}
	}
	if tag == "/*" {
		all = append(all, indent+"*/")
	}
	return strings.Join(all, "\n")
}

func classifyComment(s string) (tag, text string) {
	ns := strings.TrimSpace(s)

	if tail, ok := strings.CutPrefix(ns, "//"); ok {
		return "//", tail
	}
	if tail, ok := strings.CutPrefix(ns, "/*"); ok {
		base := strings.TrimSuffix(tail, "*/")
		return "/*", strings.TrimSpace(base)
	}
	return "??", ns
}

// trimSpaceSuffix removes whitespace from the suffix of s.
func trimSpaceSuffix(s string) string { return strings.TrimSpace("|" + s)[1:] }

// outdentCommentLines modifies lines to remove the shortest prefix of leading
// indentation that can be removed to leave the text flush left, and any
// trailing whitespace. It returns the count of indentation characters removed.
// The first line is assumed to be already cleaned of leading whitespace.
func outdentCommentLines(lines []string) int {
	// Find the shortest common prefix of the lines that can be removed to
	// leave the text flush left. Note the first line is already flush,
	// because we already trimmed it.
	pfx := -1
	for i, line := range lines[1:] {
		var ns int
		for _, c := range line {
			if c != ' ' && c != '\t' {
				break
			}
			ns++
		}
		if i == 0 || ns < pfx {
			pfx = ns
		}
	}

	// Remove the common prefix and trailing whitespace from each line.
	// Note the first line is already flush left, so skip it.
	lines[0] = trimSpaceSuffix(lines[0])
	for i, line := range lines[1:] {
		lines[i+1] = trimSpaceSuffix(line[pfx:])
	}
	return pfx
}
