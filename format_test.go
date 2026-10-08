package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jiho-dev/mygofmt/internal/config"
)

func testOptions() config.Config {
	cfg := config.Default()
	cfg.Layout.MaxLineLength = 80
	// Golden samples also exercise the optional signature and call rules.
	cfg.Layout.Wrap.FunctionParameters = true
	cfg.Layout.Wrap.FunctionResults = true
	cfg.Layout.Wrap.TypeParameters = true
	cfg.Layout.Wrap.CallArguments = true
	return cfg
}

func TestSamples(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.input.go")
	if err != nil || len(paths) < 10 {
		t.Fatalf("samples: %v (%d files)", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(path, ".input.go") + ".golden.go")
			if err != nil {
				t.Fatal(err)
			}
			got, err := Format(input, testOptions())
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("unexpected output:\n%s\nwant:\n%s", got, want)
			}
			verifyInvariants(t, input, got, testOptions())
		})
	}
}

func verifyInvariants(t *testing.T, input, output []byte, options config.Config) {
	t.Helper()
	baseline, err := format.Source(input)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		next, err := format.Source(baseline)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(baseline, next) {
			break
		}
		baseline = next
	}
	again, err := Format(output, options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, again) {
		t.Fatalf("not idempotent:\n%s", again)
	}
	standard, err := format.Source(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(output, standard) {
		t.Fatal("not gofmt compatible")
	}
	if astSnapshot(t, baseline) != astSnapshot(t, output) {
		t.Fatal("formatter changed the AST")
	}
	if !reflect.DeepEqual(comments(t, baseline), comments(t, output)) {
		t.Fatal("formatter changed comments")
	}
}

func astSnapshot(t *testing.T, source []byte) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	filter := func(name string, value reflect.Value) bool {
		return value.Type() != reflect.TypeOf(token.Pos(0)) && name != "Obj" && name != "Scope" && name != "Unresolved" && name != "FileStart" && name != "FileEnd"
	}
	if err := ast.Fprint(&out, nil, file, filter); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func comments(t *testing.T, source []byte) []string {
	t.Helper()
	file := token.NewFileSet().AddFile("", -1, len(source))
	var scan scanner.Scanner
	scan.Init(file, source, nil, scanner.ScanComments)
	var result []string
	for {
		_, tok, literal := scan.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.COMMENT {
			result = append(result, literal)
		}
	}
	return result
}

func TestInvalidInputs(t *testing.T) {
	for _, source := range []string{"package", "package p\nfunc f( {", "package p\nfunc f() { \"unterminated }"} {
		if _, err := Format([]byte(source), testOptions()); err == nil {
			t.Fatalf("accepted invalid source %q", source)
		}
	}
	bad := testOptions()
	bad.Layout.MaxLineLength = 0
	if _, err := Format([]byte("package p"), bad); err == nil {
		t.Fatal("accepted zero line width")
	}
}

func TestCustomLogMethods(t *testing.T) {
	input := []byte("package p\nfunc f() { prepare(); audit.Record(\"event\"); finish() }\n")
	options := testOptions()
	options.Calls[0].Methods = []string{"Record"}
	got, err := Format(input, options)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("prepare()\n\n\taudit.Record(\"event\")\n\n\tfinish()")) {
		t.Fatalf("custom log method not spaced:\n%s", got)
	}
	verifyInvariants(t, input, got, options)
}

func TestCLI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.go")
	input := []byte("package p\nfunc f(){log.Info(\"hello\")}\n")
	if err := os.WriteFile(path, input, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-check", path}, strings.NewReader(""), &stdout, &stderr); code != 1 {
		t.Fatalf("check exit=%d: %s", code, &stderr)
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatal("check did not list changed file")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(input, unchanged) {
		t.Fatal("check modified file")
	}
	if code := run([]string{"-w", path}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("write exit=%d: %s", code, &stderr)
	}
	if code := run([]string{"-check", path}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("formatted file failed check: %s", &stderr)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions changed: %v", err)
	}
	for _, args := range [][]string{{"-w"}, {"-w", "-check", path}, {"-m", "0", path}, {"-tab-width", "0", path}, {"-unknown"}, {filepath.Join(dir, "missing.go")}} {
		if code := run(args, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("%v: exit=%d", args, code)
		}
	}
	if code := run(nil, bytes.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("stdin failed: %s", &stderr)
	}
}

func TestBatchSyntaxErrorDoesNotWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	valid := filepath.Join(dir, "a.go")
	input := []byte("package p\nfunc f(){log.Info(\"hello\")}\n")
	if err := os.WriteFile(valid, input, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-w", dir}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
		t.Fatalf("exit=%d", code)
	}
	got, err := os.ReadFile(valid)
	if err != nil || !bytes.Equal(got, input) {
		t.Fatal("syntax error partially wrote batch")
	}
}

func TestCollectFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "nested/b.go", "vendor/c.go", "testdata/d.go", ".git/e.go", "notes.txt"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("package p"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths, err := collectFiles([]string{dir, filepath.Join(dir, "a.go")})
	if err != nil || len(paths) != 2 {
		t.Fatalf("files=%v err=%v", paths, err)
	}
	paths, err = collectFiles([]string{filepath.Join(dir, "testdata")})
	if err != nil || len(paths) != 1 {
		t.Fatalf("explicit testdata=%v err=%v", paths, err)
	}
	link := filepath.Join(dir, "linked.go")
	if err := os.Symlink(filepath.Join(dir, "a.go"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := collectFiles([]string{link}); err == nil {
		t.Fatal("accepted symlink")
	}
	paths, err = collectFiles([]string{dir})
	if err != nil || len(paths) != 2 {
		t.Fatalf("directory containing symlink: %v, %v", paths, err)
	}
}

func FuzzFormat(f *testing.F) {
	paths, _ := filepath.Glob("testdata/*.input.go")
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		if _, err := parser.ParseFile(token.NewFileSet(), "", source, parser.ParseComments); err != nil {
			return
		}
		output, err := Format(source, testOptions())
		if err != nil {
			t.Fatal(err)
		}
		verifyInvariants(t, source, output, testOptions())
	})
}

func TestWrappingKeepsShortNestedExpressions(t *testing.T) {
	input := []byte(`package p
func f() {
    flag.String("resource-type", string(sampletypes.ResourceTypeExample), "Example or Other")
    if firstType != reflect.TypeOf(second.err) || !reflect.ValueOf(first.err).Comparable() || !reflect.ValueOf(second.err).Comparable() {
        return
    }
}
`)
	options := testOptions()
	got, err := Format(input, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []string{"string(sampletypes.ResourceTypeExample)", "reflect.TypeOf(second.err)", "reflect.ValueOf(first.err).Comparable()", "reflect.ValueOf(second.err).Comparable()"} {
		if !bytes.Contains(got, []byte(expression)) {
			t.Errorf("short nested expression was split: %s\n%s", expression, got)
		}
	}
	verifyInvariants(t, input, got, options)
}
