package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type replacement struct {
	path, temporary string
	original        [sha256.Size]byte
	info            os.FileInfo
}

// Prepare in the destination directory so rename stays on one filesystem.
func prepareReplacement(path string, original []byte, output io.Reader) (*replacement, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", path)
	}

	file, err := os.CreateTemp(filepath.Dir(path), ".mygofmt-*")
	if err != nil {
		return nil, err
	}

	item := &replacement{path: path, temporary: file.Name(), original: sha256.Sum256(original), info: info}
	ready := false
	defer func() {
		if !ready {
			file.Close()
			item.cleanup()
		}
	}()
	if _, err := io.Copy(file, output); err != nil {
		return nil, err
	}

	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return nil, err
	}

	if err := file.Sync(); err != nil {
		return nil, err
	}

	if err := file.Close(); err != nil {
		return nil, err
	}

	ready = true

	return item, nil
}

func (r *replacement) cleanup() {
	if r.temporary != "" {
		os.Remove(r.temporary)
	}
}

func (r *replacement) commit() error {
	info, err := os.Lstat(r.path)
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() || !os.SameFile(r.info, info) || info.Mode() != r.info.Mode() {
		return fmt.Errorf("file changed during formatting: %s", r.path)
	}

	current, err := os.ReadFile(r.path)
	if err != nil {
		return err
	}

	if sha256.Sum256(current) != r.original {
		return fmt.Errorf("file changed during formatting: %s", r.path)
	}

	if err := os.Rename(r.temporary, r.path); err != nil {
		return err
	}

	r.temporary = ""

	return nil
}
