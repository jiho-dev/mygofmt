package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jiho-dev/mygofmt/internal/config"
)

func TestFormattingExceptions(t *testing.T) {
	input := []byte(`package p
func f() {
	// mygofmt:ignore
	value := []int{1,2,3}
	// mygofmt:off
	if true { log.Info("keep") }
	// mygofmt:on
	result:=value
	_ = result
}
`)
	for _, cfg := range []config.Config{config.Default(), loadExampleConfig(t)} {
		got, err := Format(input, cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, kept := range []string{
			"\tvalue := []int{1,2,3}\n",
			"\tif true { log.Info(\"keep\") }\n",
		} {
			if !bytes.Contains(got, []byte(kept)) {
				t.Fatalf("exception was formatted: %q\n%s", kept, got)
			}
		}
		if !bytes.Contains(got, []byte("result := value")) || bytes.Contains(got, []byte("__internal-line-end")) {
			t.Fatalf("ordinary code was not formatted or marker leaked:\n%s", got)
		}
		again, err := Format(got, cfg)
		if err != nil || !bytes.Equal(got, again) {
			t.Fatalf("exception result is unstable: %v\n%s\nnext:\n%s", err, got, again)
		}
	}
}

func TestIgnoreFileReturnsOriginal(t *testing.T) {
	input := []byte("// mygofmt:ignore-file\npackage p\nfunc f(){ x:=1;_ = x }\n")
	got, err := Format(input, loadExampleConfig(t))
	if err != nil || !bytes.Equal(got, input) {
		t.Fatalf("file exception changed source: %v\n%s", err, got)
	}
}

func TestMultipleIgnoredLines(t *testing.T) {
	input := []byte("package p\n\nfunc f() {\n\t// mygofmt:ignore\n\tx:=1\n\t// mygofmt:ignore\n\ty:=2\n\t_ = x\n\t_ = y\n}\n")
	got, err := Format(input, loadExampleConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("\tx:=1\n")) || !bytes.Contains(got, []byte("\ty:=2\n")) {
		t.Fatalf("one of the ignored lines changed:\n%s", got)
	}
	again, err := Format(got, loadExampleConfig(t))
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("multiple ignored lines are unstable: %v\n%s\nnext:\n%s", err, got, again)
	}
}

func TestIgnoredLastLineWithoutNewline(t *testing.T) {
	input := []byte("package p\n// mygofmt:ignore\nvar x=1")
	got, err := Format(input, config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(got, []byte("var x=1")) {
		t.Fatalf("last line changed:\n%s", got)
	}
}

func TestExceptionDirectivesAreComments(t *testing.T) {
	input := []byte("package p\nvar message = `// mygofmt:ignore-file`\nfunc f(){x:=1;_ = x}\n")
	got, err := Format(input, config.Default())
	if err != nil || !bytes.Contains(got, []byte("func f() {")) {
		t.Fatalf("raw string was interpreted as directive: %v\n%s", err, got)
	}
}

func TestInvalidFormattingExceptions(t *testing.T) {
	for name, source := range map[string]string{
		"unclosed region":   "package p\n// mygofmt:off\nfunc f() {}\n",
		"unmatched on":      "package p\n// mygofmt:on\nfunc f() {}\n",
		"nested region":     "package p\n// mygofmt:off\n// mygofmt:off\n// mygofmt:on\n",
		"inline directive":  "package p\nfunc f() { call() // mygofmt:ignore\n}\n",
		"misplaced file":    "package p\n// mygofmt:ignore-file\nfunc f() {}\n",
		"missing line":      "package p\n// mygofmt:ignore\n",
		"empty line":        "package p\n// mygofmt:ignore\n\nvar x = 1\n",
		"ignore in region":  "package p\n// mygofmt:off\n// mygofmt:ignore\nvar x = 1\n// mygofmt:on\n",
		"unknown directive": "package p\n// mygofmt:ignroe\nfunc f() {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Format([]byte(source), config.Default()); err == nil {
				t.Fatal("invalid exception directive was accepted")
			}
		})
	}
}

func TestCLIIgnoredLineCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "sample.go")
	input := []byte("package p\n\nfunc f() {\n\t// mygofmt:ignore\n\tx:=1\n\t_ = x\n}\n")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-check", path}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("ignored line failed check (%d): %s\n%s", code, &stdout, &stderr)
	}
}

func loadExampleConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load("mygofmt.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
