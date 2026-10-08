package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePartialDefaults(t *testing.T) {
	cfg, err := Parse([]byte("version: 1\nlayout:\n  max-line-length: 80\n  wrap:\n    call-arguments: false\nwhitespace:\n  before-return: false\ncalls: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Layout.MaxLineLength != 80 || cfg.Layout.Wrap.CallArguments || cfg.Whitespace.BeforeReturn || len(cfg.Calls) != 0 {
		t.Fatalf("overrides lost: %+v", cfg)
	}
	if !cfg.Layout.Wrap.CompositeLiterals || cfg.Layout.TabWidth != 4 || cfg.Base.Formatter != "gofmt" {
		t.Fatal("defaults lost")
	}
}

func TestRejectInvalidConfig(t *testing.T) {
	cases := map[string]string{
		"empty":              "",
		"scalar":             "hello",
		"null":               "version: null",
		"unknown":            "layuot: {}",
		"nested unknown":     "layout: {wrap: {calls: true}}",
		"duplicate key":      "version: 1\nversion: 1",
		"multiple documents": "version: 1\n---\nversion: 1",
		"version":            "version: 2",
		"length":             "layout: {max-line-length: 0}",
		"tab":                "layout: {tab-width: 0}",
		"defer":              "layout: {defer-function-body: inline}",
		"error adjacency":    "whitespace: {call-error-check: inline}",
		"engine":             "base: {formatter: custom}",
		"gofumpt consent":    "base: {formatter: gofumpt}",
		"lang":               "base: {gofumpt: {lang-version: invalid}}",
		"generated policy":   "files: {generated: apply}",
		"line policy":        "files: {line-directives: apply}",
		"empty methods":      "calls: [{methods: []}]",
		"bad method":         "calls: [{methods: ['log.Info']}]",
		"keyword method":     "calls: [{methods: ['if']}]",
		"duplicate method":   "calls: [{methods: [Info]}, {methods: [Info]}]",
		"wrong bool":         "whitespace: {before-return: 1}",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(input)); err == nil {
				t.Fatalf("accepted %q", input)
			}
		})
	}
}

func TestGofumptConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := Parse([]byte("base:\n  formatter: gofumpt\n  allow-syntax-changes: true\n  gofumpt:\n    lang-version: go1.25\n    extra:\n      group-params: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Base.Gofumpt.Extra.GroupParams || cfg.Base.Gofumpt.Extra.ClotheReturns {
		t.Fatal("extra rule overrides lost")
	}
	if _, err := Load("does-not-exist.yaml"); err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatal("missing config accepted")
	}
	if cfg, err := Load(""); err != nil || cfg.Version != 1 {
		t.Fatal("default config failed")
	}
}

func TestDefaultConfigLocation(t *testing.T) {
	userDirectory := t.TempDir()
	t.Setenv("HOME", userDirectory)
	// The requested location is fixed, even if XDG_CONFIG_HOME is set.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := Load("")
	if err != nil || cfg.Layout.MaxLineLength != 120 {
		t.Fatalf("missing default fallback: %+v %v", cfg, err)
	}
	path := filepath.Join(userDirectory, ".config", "mygofmt.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("layout: {max-line-length: 80}"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load("")
	if err != nil || cfg.Layout.MaxLineLength != 80 {
		t.Fatalf("default file not loaded: %+v %v", cfg, err)
	}
	explicit := filepath.Join(userDirectory, "explicit.yaml")
	if err := os.WriteFile(explicit, []byte("layout: {max-line-length: 200}"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(explicit)
	if err != nil || cfg.Layout.MaxLineLength != 200 {
		t.Fatalf("explicit override: %+v %v", cfg, err)
	}
	if _, err := Load(filepath.Join(userDirectory, "missing.yaml")); err == nil {
		t.Fatal("explicit missing file fell back")
	}
	if err := os.WriteFile(path, []byte("unknown: true"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil {
		t.Fatal("invalid default file silently ignored")
	}
	if _, err := Load(explicit); err != nil {
		t.Fatalf("invalid default affected explicit path: %v", err)
	}
}

func TestDefaultsAreIndependent(t *testing.T) {
	a := Default()
	a.Calls[0].Methods[0] = "Changed"
	if Default().Calls[0].Methods[0] == "Changed" {
		t.Fatal("shared mutable defaults")
	}
}

func TestCallReceiverValidation(t *testing.T) {
	for _, receivers := range []string{"['']", "['fl.']", "['fl..logger']", "['for']", "['_']", "['makeLogger()']", "['fl', 'fl']"} {
		if _, err := Parse([]byte("calls:\n  - methods: [Error]\n    receivers: " + receivers + "\n")); err == nil {
			t.Errorf("accepted invalid receivers: %s", receivers)
		}
	}
	cfg, err := Parse([]byte("calls:\n  - methods: [Error]\n    receivers: [fl, s.logger]\n"))
	if err != nil || len(cfg.Calls[0].Receivers) != 2 {
		t.Fatalf("valid receiver configuration: %v, %v", cfg, err)
	}
}

func TestMinimumCallArgumentsConfig(t *testing.T) {
	if _, err := Parse([]byte("layout:\n  min-call-arguments: 0\n")); err == nil {
		t.Fatal("accepted zero minimum call arguments")
	}
	cfg, err := Parse([]byte("layout:\n  min-call-arguments: 3\n"))
	if err != nil || cfg.Layout.MinCallArguments != 3 {
		t.Fatalf("minimum call arguments: %v, %v", cfg, err)
	}
	cfg = Default()
	if cfg.Layout.MinCallArguments != 2 || cfg.Layout.Wrap.FunctionParameters || cfg.Layout.Wrap.FunctionResults || cfg.Layout.Wrap.TypeParameters || cfg.Layout.Wrap.CallArguments {
		t.Fatalf("unexpected signature or call defaults: %+v", cfg.Layout)
	}
}
