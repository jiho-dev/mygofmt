// Package formatter composes an upstream base and configuration-driven layout.
package formatter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/jiho-dev/mygofmt/internal/base"
	"github.com/jiho-dev/mygofmt/internal/config"
	"github.com/jiho-dev/mygofmt/internal/layoutstyle"
)

func Format(source []byte, cfg config.Config) ([]byte, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	plan, err := parseExceptions(source, fset, file)
	if err != nil {
		return nil, err
	}
	if plan.skipFile {
		return bytes.Clone(source), nil
	}
	output, err := formatCore(plan.source, cfg)
	if err != nil {
		return nil, err
	}
	return plan.restore(output)
}

func formatCore(source []byte, cfg config.Config) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	policy := ""
	if ast.IsGenerated(file) {
		policy = cfg.Files.Generated
	}
	if hasLineDirective(file) && policy != "skip" {
		policy = cfg.Files.LineDirectives
	}
	if policy == "skip" {
		return bytes.Clone(source), nil
	}
	if policy == "gofmt-only" {
		return base.Gofmt(source)
	}
	for round := 0; round < 8; round++ {
		normalized, err := base.Format(source, cfg.Base)
		if err != nil {
			return nil, fmt.Errorf("base formatter: %w", err)
		}
		if cfg.Base.Formatter == "gofumpt" {
			original, err := base.Gofmt(source)
			if err != nil {
				return nil, fmt.Errorf("format original signatures: %w", err)
			}
			normalized = restoreBlockCommentGaps(original, normalized)
			normalized, err = restoreSignatureLayout(original, normalized, cfg.Layout.Wrap)
			if err != nil {
				return nil, fmt.Errorf("restore signature layout: %w", err)
			}
		}
		output, err := layoutstyle.Apply(normalized, cfg)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(source, output) {
			return output, nil
		}
		source = output
	}
	return nil, fmt.Errorf("combined formatting did not converge after 8 rounds")
}

func hasLineDirective(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//line ") || strings.HasPrefix(comment.Text, "/*line ") {
				return true
			}
		}
	}
	return false
}
