package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/jiho-dev/mygofmt/internal/config"
)

func TestYAMLRuleSwitchesChangeFormatting(t *testing.T) {
	cases := []struct {
		name, source, otherConfig, marker string
		markerWhenOn                      bool
	}{
		{
			name: "wrap-function-parameters", source: "package p\nfunc f(first VeryLongType, second VeryLongType) {}\n",
			otherConfig: "layout: {max-line-length: 40}\n", marker: "func f(\n", markerWhenOn: true,
		},
		{
			name: "wrap-function-results", source: "package p\nfunc f() (FirstLongResult, SecondLongResult) { panic(0) }\n",
			otherConfig: "layout: {max-line-length: 40}\n", marker: "func f() (\n", markerWhenOn: true,
		},
		{
			name: "wrap-type-parameters", source: "package p\nfunc f[First VeryLongConstraint, Second VeryLongConstraint]() {}\n",
			otherConfig: "layout: {max-line-length: 40}\n", marker: "func f[\n", markerWhenOn: true,
		},
		{
			name:         "wrap-call-arguments",
			source:       "package p\nfunc f() { send(longFirstArgument, longSecondArgument) }\n",
			otherConfig:  "layout: {max-line-length: 40}\n",
			marker:       "send(\n",
			markerWhenOn: true,
		},
		{
			name: "wrap-composite-literals", source: "package p\nfunc f() { _ = Result{First: longFirstValue, Second: longSecondValue} }\n",
			otherConfig: "layout: {max-line-length: 45}\n", marker: "Result{\n", markerWhenOn: true,
		},
		{
			name: "wrap-nested-composite-literals", source: "package p\nvar _ = Outer{\nInner: Inner{First: a, Second: b},\n}\n",
			marker: "Inner: Inner{\n", markerWhenOn: true,
		},
		{
			name: "balance-logical-conditions", source: "package p\nfunc f() { if firstLongCondition && secondLongCondition && thirdLongCondition { run() } }\n",
			otherConfig: "layout: {max-line-length: 45}\n", marker: "&&\n", markerWhenOn: true,
		},
		{
			name: "multiline-defer-body", source: "package p\nfunc f() { defer func() { cleanup() }() }\n",
			marker: "defer func() {\n", markerWhenOn: true,
		},
		{
			name: "preserve-existing-blank-lines", source: "package p\nfunc f() {\nfirst()\n\nsecond()\n}\n",
			marker: "first()\n\n\tsecond()", markerWhenOn: true,
		},
		{
			name:         "blank-before-return",
			source:       "package p\nfunc f() int { work(); return 1 }\n",
			marker:       "work()\n\n\treturn",
			markerWhenOn: true,
		},
		{
			name: "blank-before-loop", source: "package p\nfunc f() { prepare(); for ok { run() } }\n",
			marker: "prepare()\n\n\tfor", markerWhenOn: true,
		},
		{
			name: "blank-after-loop", source: "package p\nfunc f() { for ok { run() }; finish() }\n",
			marker: "}\n\n\tfinish()", markerWhenOn: true,
		},
		{
			name: "blank-after-control-block", source: "package p\nfunc f() { if ok { run() }; finish() }\n",
			marker: "}\n\n\tfinish()", markerWhenOn: true,
		},
		{
			name: "blank-after-multiline-initialization", source: "package p\nfunc f() { r := Result{\nA: a,\n}; use(r) }\n",
			marker: "}\n\n\tuse(r)", markerWhenOn: true,
		},
		{
			name: "join-call-error-checks", source: "package p\nfunc f() {\n_, err := load()\n\nif err != nil { return }\n}\n",
			marker: "load()\n\n\tif err", markerWhenOn: false,
		},
		{
			name:         "space-configured-calls",
			source:       "package p\nfunc f() { prepare(); log.Info(\"event\"); finish() }\n",
			marker:       "prepare()\n\n\tlog.Info",
			markerWhenOn: true,
		},
		{
			name:         "compact-single-statement-blocks",
			source:       "package p\nfunc f() { if ok {\n\nrun()\n\n} }\n",
			marker:       "if ok {\n\n",
			markerWhenOn: false,
		},
		{
			name: "preserve-block-comment-gaps", source: "package p\nfunc f() {\nx, err := start()\n\n/*\nnote\n*/\n\nif err != nil { return }\n_ = x\n}\n",
			otherConfig: "base: {formatter: gofumpt, allow-syntax-changes: true}\n", marker: "start()\n\n\t/*", markerWhenOn: true,
		},
		{
			name: "preserve-signature-layout", source: "package p\nfunc (worker *Worker) process(ctx Context, logger Logger,\n request Request, key, actor string) (result Result, err error) { return }\n",
			otherConfig: "base: {formatter: gofumpt, allow-syntax-changes: true}\n", marker: "key, actor string)", markerWhenOn: true,
		},
		{
			name:         "gofumpt-group-params",
			source:       "package p\nfunc f(a int, b int) {}\n",
			otherConfig:  "base: {formatter: gofumpt, allow-syntax-changes: true}\n",
			marker:       "func f(a, b int)",
			markerWhenOn: true,
		},
		{
			name: "gofumpt-clothe-returns", source: "package p\nfunc f() (n int) { n = 1; return }\n",
			otherConfig: "base: {formatter: gofumpt, allow-syntax-changes: true}\n", marker: "return n", markerWhenOn: true,
		},
		{
			name: "gofumpt-balance-calls", source: "package p\nfunc f() { send(\nvalue) }\n",
			otherConfig: "base: {formatter: gofumpt, allow-syntax-changes: true}\n", marker: "value,\n\t)", markerWhenOn: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, enabled := range []bool{false, true} {
				cfg, err := config.Parse([]byte(tc.otherConfig + fmt.Sprintf("rules: {%s: %t}\n", tc.name, enabled)))
				if err != nil {
					t.Fatal(err)
				}
				got, err := Format([]byte(tc.source), cfg)
				if err != nil {
					t.Fatal(err)
				}
				if present := bytes.Contains(got, []byte(tc.marker)); present != (enabled == tc.markerWhenOn) {
					t.Fatalf("enabled=%t marker %q present=%t:\n%s", enabled, tc.marker, present, got)
				}
			}
		})
	}
}

func TestYAMLSchemaRejectsOldRuleLocations(t *testing.T) {
	for _, source := range []string{
		"layout: {wrap: {call-arguments: true}}",
		"layout: {defer-function-body: preserve}",
		"whitespace: {before-return: false}",
		"base: {gofumpt: {extra: {group-params: true}}}",
	} {
		if _, err := config.Parse([]byte(source)); err == nil || !strings.Contains(err.Error(), "rules") {
			t.Fatalf("old rule location %q was not directed to rules: %v", source, err)
		}
	}
	if cfg, err := config.Parse([]byte("version: 1\nrules: {blank-before-return: false}\n")); err != nil || cfg.Version != 1 || cfg.Whitespace.BeforeReturn {
		t.Fatalf("version 1 rules were not accepted: %+v, %v", cfg, err)
	}
}
