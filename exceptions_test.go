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

	for _, cfg := range []config.Config{config.Default(), gofumptTestConfig(t)} {
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
	got, err := Format(input, gofumptTestConfig(t))
	if err != nil || !bytes.Equal(got, input) {
		t.Fatalf("file exception changed source: %v\n%s", err, got)
	}
}

func TestMultipleIgnoredLines(t *testing.T) {
	input := []byte("package p\n\nfunc f() {\n\t// mygofmt:ignore\n\tx:=1\n\t// mygofmt:ignore\n\ty:=2\n\t_ = x\n\t_ = y\n}\n")
	got, err := Format(input, gofumptTestConfig(t))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(got, []byte("\tx:=1\n")) || !bytes.Contains(got, []byte("\ty:=2\n")) {
		t.Fatalf("one of the ignored lines changed:\n%s", got)
	}

	again, err := Format(got, gofumptTestConfig(t))
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

func gofumptTestConfig(t *testing.T) config.Config {
	t.Helper()
	cfg := config.Default()
	cfg.Base.Formatter = "gofumpt"
	cfg.Base.AllowSyntaxChanges = true

	return cfg
}

func TestIgnoreMultilineTokens(t *testing.T) {
	for name, protected := range map[string]string{
		"raw string":       "var s=`first\nsecond`\n",
		"block comment":    "/* first\n  second */\n",
		"chained tokens":   "var s=`first\nsecond` + `third\nfourth`\n",
		"CRLF":             "var s=`first\r\nsecond`\r\n",
		"no final newline": "var s=`first\nsecond`",
	} {
		t.Run(name, func(t *testing.T) {
			for _, cfg := range []config.Config{config.Default(), gofumptTestConfig(t)} {
				input := []byte("package p\nvar ordinary=1\n// mygofmt:ignore\n" + protected)
				got, err := Format(input, cfg)
				if err != nil {
					t.Fatal(err)
				}

				if !bytes.Contains(got, []byte(protected)) || !bytes.Contains(got, []byte("ordinary = 1")) ||
					bytes.Contains(got, []byte("__internal-line-end")) {
					t.Fatalf("exception lost or ordinary code not formatted:\n%s", got)
				}

				again, err := Format(got, cfg)
				if err != nil || !bytes.Equal(got, again) {
					t.Fatalf("unstable exception: %v\n%s\n%s", err, got, again)
				}
			}
		})
	}
}

func TestProtectedCompilerDirectives(t *testing.T) {
	for _, directive := range []string{
		"//go:noinline",
		"//go:linkname f example.com/p.target",
		"//go:embed data.txt",
		"//nolint:unused",
		"//lint:ignore U1000 reason",
		"//export f",
		"//extern f",
		"//custom:directive",
	} {
		for _, region := range []bool{false, true} {
			protected := directive + "\n"
			prefix, suffix := "// mygofmt:ignore\n", ""
			if region {
				prefix, suffix = "// mygofmt:off\n", "// mygofmt:on\n"
				protected = "// explanation\n" + protected
			}

			declaration := "func f(){}\n"
			if strings.Contains(directive, "embed") {
				declaration = "var data string\n"
			}

			input := []byte("package p\n" + prefix + protected + suffix + declaration)

			for _, cfg := range []config.Config{config.Default(), gofumptTestConfig(t)} {
				got, err := Format(input, cfg)
				if err != nil {
					t.Fatal(err)
				}

				if bytes.Count(got, []byte(directive)) != 1 || !bytes.Contains(got, []byte(protected)) {
					t.Fatalf("directive duplicated or changed:\n%s", got)
				}

				again, err := Format(got, cfg)
				if err != nil || !bytes.Equal(got, again) {
					t.Fatalf("unstable directive: %v\n%s\n%s", err, got, again)
				}
			}
		}
	}
}

func TestProtectedLineDirectivePolicy(t *testing.T) {
	input := []byte("package p\n// mygofmt:ignore\n//line original.go:10\nfunc f(){work();return}\n")
	cfg := config.Default()
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(got, []byte("//line original.go:10\n")) || bytes.Contains(got, []byte("\n\n\treturn")) {
		t.Fatalf("line directive or gofmt-only policy lost:\n%s", got)
	}

	again, err := Format(got, cfg)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("unstable line directive: %v\n%s", err, again)
	}

	cfg.Files.LineDirectives = "skip"
	got, err = Format(input, cfg)
	if err != nil || !bytes.Equal(input, got) {
		t.Fatalf("skip policy lost: %v\n%s", err, got)
	}
}
