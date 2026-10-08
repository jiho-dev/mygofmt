// Package base adapts upstream formatters without modifying their sources.
package base

import (
	"bytes"
	"fmt"
	"go/format"

	"github.com/jiho-dev/mygofmt/internal/config"
	fumpt "mvdan.cc/gofumpt/format"
)

func Format(source []byte, cfg config.Base) ([]byte, error) {
	source, err := Gofmt(source)
	if err != nil {
		return nil, err
	}
	if cfg.Formatter == "gofmt" {
		return source, nil
	}
	if cfg.Formatter != "gofumpt" {
		return nil, fmt.Errorf("unknown base formatter: %s", cfg.Formatter)
	}
	if !cfg.AllowSyntaxChanges {
		return nil, fmt.Errorf("gofumpt requires allow-syntax-changes")
	}
	return fumpt.Source(source, fumpt.Options{
		LangVersion: cfg.Gofumpt.LangVersion,
		ModulePath:  cfg.Gofumpt.ModulePath,
		Extra:       fumpt.Extra{GroupParams: cfg.Gofumpt.Extra.GroupParams, ClotheReturns: cfg.Gofumpt.Extra.ClotheReturns, BalanceCalls: cfg.Gofumpt.Extra.BalanceCalls},
	})
}

// Gofmt also normalizes malformed //line-like ordinary comments, which can
// require two standard formatting passes to reach their final spelling.
func Gofmt(source []byte) ([]byte, error) {
	for round := 0; round < 8; round++ {
		output, err := format.Source(source)
		if err != nil {
			return nil, err
		}
		if bytes.Equal(source, output) {
			return output, nil
		}
		source = output
	}
	return nil, fmt.Errorf("gofmt did not converge after 8 rounds")
}
