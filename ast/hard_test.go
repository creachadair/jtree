// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package ast_test

import (
	"archive/zip"
	"errors"
	"flag"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/creachadair/jtree/ast"
)

var (
	doHardTest = flag.Bool("compliance-test", false,
		"Run full compliance test")
	hardTestURL = flag.String("compliance-test-repo", "https://github.com/nst/JSONTestSuite",
		"Compliance test repository URL")

	// The tests exercised here are those described by the article "Parsing JSON
	// is a Minefield", https://seriot.ch/projects/parsing_json.html.
	//
	// The test explicitly checks the affirmative (y_*) and negative (n_*)
	// cases, but does not exercise the indeterminate (i_*) cases.
)

func mustGetArchive(t *testing.T, zipFile string) *zip.Reader {
	t.Helper()

	if fi, err := os.Stat(zipFile); err == nil {
		zf, err := os.Open(zipFile)
		if err != nil {
			t.Fatalf("Open archive: %v", err)
		}
		t.Cleanup(func() { zf.Close() })
		zr, err := zip.NewReader(zf, fi.Size())
		if err != nil {
			t.Fatalf("Open reader: %v", err)
		}
		return zr
	} else if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Stat archive: %v", err)
	}

	fullURL := *hardTestURL + "/archive/refs/heads/master.zip"
	t.Logf("Fetching %q ...", fullURL)
	rsp, err := http.Get(fullURL)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}
	defer rsp.Body.Close()
	if ctype := rsp.Header.Get("content-type"); ctype != "application/zip" {
		t.Fatalf("Unexpected content-type: %q", ctype)
	}

	zf, err := os.Create(zipFile)
	if err != nil {
		t.Fatalf("Create output: %v", err)
	}
	t.Cleanup(func() { zf.Close() })

	size, err := io.Copy(zf, rsp.Body)
	if err != nil {
		t.Fatalf("Write output: %v", err)
	}
	zr, err := zip.NewReader(zf, size)
	if err != nil {
		t.Fatalf("Open reeader: %v", err)
	}
	return zr
}

func mustFetchTestFiles(t *testing.T, fn func(*zip.File) error) {
	t.Helper()

	zr := mustGetArchive(t, "hard-test-suite.zip")

	for _, file := range zr.File {
		if err := fn(file); err != nil {
			t.Fatalf("File %q: %v", file.Name, err)
		}
	}
}

// mustParse fully reads the contents of zf and parses it.
// An error from parsing is returned; errors from reading fail the test.
func mustParse(t *testing.T, zf *zip.File) (ast.Value, error) {
	t.Helper()
	rc, err := zf.Open()
	if err != nil {
		t.Fatalf("Open %q: %v", zf.Name, err)
	}
	defer rc.Close()
	return ast.ParseSingle(rc)
}

func TestCompliance(t *testing.T) {
	if !*doHardTest {
		t.Skip("Skipping compliance test because --compliance-test is false")
	}
	var numYes, numYesErrs, numNo, numNoErrs int
	mustFetchTestFiles(t, func(f *zip.File) error {
		_, tail, ok := strings.Cut(f.Name, "/test_parsing/")
		if !ok || filepath.Ext(tail) != ".json" {
			return nil
		}
		tail = strings.TrimSuffix(tail, filepath.Ext(tail))
		tag, _, _ := strings.Cut(tail, "_")
		switch tag {
		case "y":
			numYes++
			t.Run(tail, func(t *testing.T) {
				if _, err := mustParse(t, f); err != nil {
					numYesErrs++
					t.Errorf("Test %q: unexpected error: %v", tail, err)
				}
			})
		case "n":
			numNo++
			t.Run(tail, func(t *testing.T) {
				if v, err := mustParse(t, f); err == nil {
					numNoErrs++
					t.Errorf("Test %q: wanted error\n%v", tail, v)
				} else {
					t.Logf("- [expected]: %v", err)
				}
			})
		case "i":
			// OK, skip silently
		default:
			t.Logf("WARNING: Skipped non-maching filename %q", tail)
		}
		return nil
	})
	t.Logf("Ran %d positive tests, %d errors", numYes, numYesErrs)
	t.Logf("Ran %d negative tests, %d errors", numNo, numNoErrs)
}
