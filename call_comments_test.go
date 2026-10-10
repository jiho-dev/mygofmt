package main

import (
	"strings"
	"testing"

	"github.com/jiho-dev/mygofmt/internal/config"
)

func TestConfiguredCallComments(t *testing.T) {
	cases := []struct {
		name, body, want   string
		disable, afterOnly bool
	}{
		{name: "single statement", body: "// explains event\n\nlogger.Info(\"event\")", want: "// explains event\n\tlogger.Info"},
		{name: "gap before comment", body: "prepare()\n// explains event\n\nlogger.Info(\"event\")\nfinish()", want: "prepare()\n\n\t// explains event\n\tlogger.Info"},
		{name: "after only", body: "prepare()\n// explains event\n\nlogger.Info(\"event\")\nfinish()", want: "prepare()\n\t// explains event\n\tlogger.Info", afterOnly: true},
		{name: "block comment", body: "/* explains\n event */\n\nlogger.Info(\"event\")", want: "event */\n\tlogger.Info"},
		{name: "comment paragraphs", body: "// overview\n\n// explains event\n\nlogger.Info(\"event\")", want: "// overview\n\n\t// explains event\n\tlogger.Info"},
		{name: "trailing line comment", body: "prepare() // previous work\n\nlogger.Info(\"event\")", want: "prepare() // previous work\n\n\tlogger.Info"},
		{name: "trailing block comment", body: "prepare() /* previous\n work */\n\nlogger.Info(\"event\")", want: "work */\n\n\tlogger.Info"},
		{name: "trailing call comment", body: "logger.Info(\"event\")\n\n// keep going", want: "logger.Info(\"event\")\n\n\t// keep going"},
		{name: "unmatched receiver", body: "// explains event\n\nother.Info(\"event\")", want: "// explains event\n\n\tother.Info"},
		{name: "unmatched method", body: "// explains event\n\nlogger.Other(\"event\")", want: "// explains event\n\n\tlogger.Other"},
		{name: "disabled", body: "// explains event\n\nlogger.Info(\"event\")", want: "// explains event\n\n\tlogger.Info", disable: true},
		{name: "defer", body: "// explains event\n\ndefer logger.Info(\"event\")", want: "// explains event\n\tdefer logger.Info"},
		{name: "go", body: "// explains event\n\ngo logger.Info(\"event\")", want: "// explains event\n\tgo logger.Info"},
		{name: "switch case", body: "switch n {\ncase 1:\n// explains event\n\nlogger.Info(\"event\")\n}", want: "// explains event\n\t\tlogger.Info"},
		{name: "ignored", body: "\t// mygofmt:off\n\t// explains event\n\n\tlogger.Info(\"event\")\n\t// mygofmt:on", want: "// explains event\n\n\tlogger.Info"},
	}
	for _, engine := range []string{"gofmt", "gofumpt"} {
		for _, tc := range cases {
			t.Run(engine+"/"+tc.name, func(t *testing.T) {
				cfg := config.Default()
				cfg.Base.Formatter = engine
				cfg.Base.AllowSyntaxChanges = engine == "gofumpt"
				cfg.Rules.SpaceConfiguredCalls = !tc.disable
				cfg.Calls = []config.CallRule{{Methods: []string{"Info"}, Receivers: []string{"logger"}, BlankBefore: !tc.afterOnly, BlankAfter: true}}
				input := []byte("package p\nfunc f() {\n" + tc.body + "\n}\n")
				got, err := Format(input, cfg)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(got), tc.want) {
					t.Fatalf("missing %q in:\n%s", tc.want, got)
				}
				verifyInvariants(t, input, got, cfg)
			})
		}
	}
}
