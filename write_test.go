package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingIO struct{}

func (failingIO) Read([]byte) (int, error)  { return 0, errors.New("injected read failure") }
func (failingIO) Write([]byte) (int, error) { return 0, errors.New("injected write failure") }

func TestReplacement(t *testing.T) {
	for _, scenario := range []string{"success", "prepare failure", "rename failure", "concurrent edit"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "source.go")
			original := []byte("original contents")
			if err := os.WriteFile(path, original, 0640); err != nil {
				t.Fatal(err)
			}

			var output io.Reader = strings.NewReader("formatted contents")
			if scenario == "prepare failure" {
				output = io.MultiReader(strings.NewReader("partial contents"), failingIO{})
			}

			item, err := prepareReplacement(path, original, output)
			want := original
			if scenario == "prepare failure" {
				if err == nil {
					item.cleanup()

					t.Fatal("expected preparation failure")

				}
			} else {
				if err != nil {
					t.Fatal(err)
				}

				defer item.cleanup()
				before, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, original) {
					t.Fatalf("preparation changed original: %v", err)
				}

				if scenario == "rename failure" {
					if err := os.Remove(item.temporary); err != nil {
						t.Fatal(err)
					}
				}

				if scenario == "concurrent edit" {
					want = []byte("new user contents")
					if err := os.WriteFile(path, want, 0640); err != nil {
						t.Fatal(err)
					}
				}

				err = item.commit()
				if scenario == "success" {
					want = []byte("formatted contents")
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil {
					t.Fatal("expected commit failure")
				}

				item.cleanup()
			}

			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("contents=%q want=%q err=%v", got, want, err)
			}

			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0640 {
				t.Fatalf("permissions changed: %v", err)
			}

			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary file left behind: %v, %v", entries, err)
			}
		})
	}
}

func TestCLIOutputFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := "package p\nfunc f(){ work() }\n"
	path := filepath.Join(t.TempDir(), "source.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}

	for _, option := range []string{"-l", "-check"} {
		for _, fileInput := range []bool{false, true} {
			args := []string{option}
			if fileInput {
				args = append(args, path)
			}

			var stderr bytes.Buffer
			if code := run(args, strings.NewReader(source), failingIO{}, &stderr); code != 2 ||
				!strings.Contains(stderr.String(), "injected write failure") {
				t.Fatalf("args=%v code=%d stderr=%s", args, code, &stderr)
			}
		}
	}
}

func TestCLIWriteDuplicatePaths(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "source.go")
	if err := os.WriteFile(path, []byte("package p\nfunc f(){ work() }\n"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)
	var stderr bytes.Buffer
	if code := run([]string{"-w", "source.go", path}, strings.NewReader(""), io.Discard, &stderr); code != 0 {
		t.Fatalf("duplicate paths failed: code=%d stderr=%s", code, &stderr)
	}

	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file left behind: %v, %v", entries, err)
	}
}

func TestCLIWriteDuplicateAncestorSymlinks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, aliasFirst := range []bool{false, true} {
		dir := t.TempDir()
		realDir, aliasDir := filepath.Join(dir, "real"), filepath.Join(dir, "alias")
		if err := os.Mkdir(realDir, 0700); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(realDir, aliasDir); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(realDir, "source.go")
		if err := os.WriteFile(path, []byte("package p\nvar x=1\n"), 0600); err != nil {
			t.Fatal(err)
		}

		paths := []string{path, filepath.Join(aliasDir, "source.go")}
		if aliasFirst {
			paths[0], paths[1] = paths[1], paths[0]
		}

		files, err := collectFiles(paths)
		if err != nil || len(files) != 1 {
			t.Fatalf("duplicate files: %v, %v", files, err)
		}

		var stderr bytes.Buffer
		if code := run(append([]string{"-w"}, paths...), strings.NewReader(""), io.Discard, &stderr); code != 0 {
			t.Fatalf("duplicate paths failed: code=%d stderr=%s", code, &stderr)
		}

		got, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(got, []byte("var x = 1")) {
			t.Fatalf("not formatted: %q, %v", got, err)
		}

		entries, err := os.ReadDir(realDir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("temporary file left behind: %v, %v", entries, err)
		}
	}
}
