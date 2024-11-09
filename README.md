# jtree

[![GoDoc](https://img.shields.io/static/v1?label=godoc&message=reference&color=purple)](https://pkg.go.dev/github.com/creachadair/jtree)
[![CI](https://github.com/creachadair/jtree/actions/workflows/go-presubmit.yml/badge.svg?event=push&branch=main)](https://github.com/creachadair/jtree/actions/workflows/go-presubmit.yml)

This repository defines a Go module that implements an streaming JSON scanner
and parser.

The [jwcc](https://godoc.org/github.com/creachadair/jtree/jwcc) package
implements the [JSON With Commas and Comments(JWCC)](https://nigeltao.github.io/blog/2021/json-with-commas-comments.html)
extension, using the same underlying parsing machinery.
