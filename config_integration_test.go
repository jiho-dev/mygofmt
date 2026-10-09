package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jiho-dev/mygofmt/internal/base"
	"github.com/jiho-dev/mygofmt/internal/config"
	"github.com/jiho-dev/mygofmt/internal/layoutstyle"
)

func noRules() config.Config {
	cfg := testOptions()
	cfg.Layout.Wrap = config.Wrap{}
	cfg.Layout.DeferFunctionBody = "preserve"
	cfg.Whitespace = config.Whitespace{PreserveExisting: true, CallErrorCheck: "preserve"}
	cfg.Calls = nil
	return cfg
}

func TestRuleSwitches(t *testing.T) {
	cases := []struct {
		name, input, marker string
		enable              func(*config.Config)
	}{
		{"return", "func f() int { prepare(); return 1 }", "prepare()\n\n\treturn", func(c *config.Config) { c.Whitespace.BeforeReturn = true }},
		{"before loop", "func f() { prepare(); for ok { run() } }", "prepare()\n\n\tfor", func(c *config.Config) { c.Whitespace.BeforeLoop = true }},
		{"after loop", "func f() { for ok { run() }; finish() }", "}\n\n\tfinish", func(c *config.Config) { c.Whitespace.AfterLoop = true }},
		{"after control", "func f() { if ok { run() }; finish() }", "}\n\n\tfinish", func(c *config.Config) { c.Whitespace.AfterControlBlock = true }},
		{"after initialization", "func f() { r := Response{\nName: name,\n}; use(r) }", "}\n\n\tuse", func(c *config.Config) { c.Whitespace.AfterMultilineInitialization = true }},
		{"defer body", "func f() { defer func() { cleanup() }() }", "defer func() {\n", func(c *config.Config) { c.Layout.DeferFunctionBody = "multiline" }},
		{"call wrap", "func f() { allocate(request.TransactionIdentifier, request.PipelineIdentifier, request.ActorIdentifier) }", "allocate(\n", func(c *config.Config) { c.Layout.Wrap.CallArguments = true }},
		{"literal wrap", "func f() { r := Response{TransactionId: request.TransactionIdentifier, PipelineId: request.PipelineIdentifier}; _ = r }", "Response{\n", func(c *config.Config) { c.Layout.Wrap.CompositeLiterals = true }},
		{"condition wrap", "func f() { if request.TransactionId != \"\" && request.PipelineId != \"\" && request.ActorId != \"\" { run() } }", "&&\n", func(c *config.Config) { c.Layout.Wrap.LogicalConditions = true }},
		{"parameter wrap", "func f(request ExampleServiceRequest, processor ProcessorClient, configuration ExampleServiceConfiguration) {}", "func f(\n", func(c *config.Config) { c.Layout.Wrap.FunctionParameters = true }},
		{"result wrap", "func f() (ExampleServiceRequest, ExampleServiceResponse, ExampleServiceConfiguration) { panic(0) }", "func f() (\n", func(c *config.Config) { c.Layout.Wrap.FunctionResults = true }},
		{"type parameter wrap", "func f[Request ExampleServiceRequestConstraint, Response ExampleServiceResponseConstraint]() {}", "func f[\n", func(c *config.Config) { c.Layout.Wrap.TypeParameters = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte("package p\n" + tc.input + "\n")
			disabled := noRules()
			off, err := Format(input, disabled)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(off, []byte(tc.marker)) {
				t.Fatalf("disabled rule took effect:\n%s", off)
			}
			enabled := noRules()
			tc.enable(&enabled)
			on, err := Format(input, enabled)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(on, []byte(tc.marker)) {
				t.Fatalf("enabled rule did not take effect:\n%s", on)
			}
			verifyInvariants(t, input, on, enabled)
			verifyInvariants(t, input, off, disabled)
		})
	}
}

func TestCallRules(t *testing.T) {
	input := []byte("package p\nfunc f() { prepare(); audit.Record(\"event\"); finish() }\n")
	for _, before := range []bool{false, true} {
		for _, after := range []bool{false, true} {
			cfg := noRules()
			cfg.Calls = []config.CallRule{{Methods: []string{"Record"}, BlankBefore: before, BlankAfter: after}}
			got, err := Format(input, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(got, []byte("prepare()\n\n\taudit")) != before || bytes.Contains(got, []byte("Record(\"event\")\n\n\tfinish")) != after {
				t.Fatalf("before=%v after=%v:\n%s", before, after, got)
			}
			verifyInvariants(t, input, got, cfg)
		}
	}
	for _, boundary := range []bool{false, true} {
		cfg := noRules()
		cfg.Calls = []config.CallRule{{Methods: []string{"Record"}, BlankBefore: true, BlankAfter: true, IncludeBlockBoundaries: boundary}}
		got, err := Format([]byte("package p\nfunc f() { audit.Record(\"first\"); audit.Record(\"event\") }"), cfg)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(got, []byte("{\n\n\taudit")) != boundary || bytes.Contains(got, []byte("event\")\n\n}")) != boundary {
			t.Fatalf("boundary=%v:\n%s", boundary, got)
		}
	}
}

func TestErrorAdjacencyAndPreserveWhitespace(t *testing.T) {
	input := []byte("package p\nfunc f() {\n x, err := call()\n\n if err != nil { return }\n _ = x\n}\n")
	cfg := noRules()
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("call()\n\n\tif")) {
		t.Fatal("preserve policy lost empty line")
	}
	cfg.Whitespace.CallErrorCheck = "adjacent"
	got, err = Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("call()\n\n\tif")) {
		t.Fatal("adjacent policy lost")
	}
	input = []byte("package p\nfunc f() {\n a := `raw\n\ntext`\n\n b := 1\n\n // keep comment\n _ = a\n _ = b\n}\n")
	cfg = noRules()
	cfg.Whitespace.PreserveExisting = false
	got, err = Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("text`\n\n\tb")) || !bytes.Contains(got, []byte("raw\n\ntext")) || !bytes.Contains(got, []byte("b := 1\n\n\t// keep")) {
		t.Fatalf("unsafe whitespace compaction:\n%s", got)
	}
	verifyInvariants(t, input, got, cfg)
}

func TestSpecialFilePolicies(t *testing.T) {
	inputs := [][]byte{
		[]byte("// Code generated by test. DO NOT EDIT.\npackage p\nfunc f(){log.Info(\"generated\")}\n"),
		[]byte("package p\nfunc f(){\n//line original.go:20\nlog.Info(\"line\")\n}"),
		[]byte("package p\nfunc f(){/*line original.go:20*/ log.Info(\"line\")}"),
	}
	for _, input := range inputs {
		cfg := testOptions()
		cfg.Base.Formatter, cfg.Base.AllowSyntaxChanges = "gofumpt", true
		want, err := base.Gofmt(input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Format(input, cfg)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("special file not gofmt-only: %v\n%s", err, got)
		}
		cfg.Files.Generated, cfg.Files.LineDirectives = "skip", "skip"
		got, err = Format(input, cfg)
		if err != nil || !bytes.Equal(got, input) {
			t.Fatal("skip changed source")
		}
	}
	// Text inside a raw literal is not a directive.
	input := []byte("package p\nfunc f() { raw := `//line source.go:20`; log.Info(raw) }")
	got, err := Format(input, testOptions())
	if err != nil || !bytes.Contains(got, []byte("\n\n\tlog.Info")) {
		t.Fatalf("raw string misclassified: %v\n%s", err, got)
	}
}

func TestGofumptComposition(t *testing.T) {
	cfg := testOptions()
	cfg.Base.Formatter, cfg.Base.AllowSyntaxChanges = "gofumpt", true
	paths, _ := filepath.Glob("testdata/*.input.go")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Format(input, cfg)
			if err != nil {
				t.Fatal(err)
			}
			again, err := Format(got, cfg)
			if err != nil || !bytes.Equal(got, again) {
				t.Fatalf("unstable combination: %v", err)
			}
			standard, err := format.Source(got)
			if err != nil || !bytes.Equal(got, standard) {
				t.Fatal("not gofmt compatible")
			}
		})
	}
	input := []byte("package p\nimport (\"fmt\"; \"example.com/lib\")\nfunc f() { var value = 1; log.Info(value); fmt.Println(lib.Value) }")
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("\"fmt\"\n\n\t\"example.com/lib\"")) || !bytes.Contains(got, []byte("value := 1")) || !bytes.Contains(got, []byte("\n\n\tlog.Info")) {
		t.Fatalf("base or extension missing:\n%s", got)
	}
	baseline, err := base.Format(input, cfg.Base)
	if err != nil {
		t.Fatal(err)
	}
	extended, err := layoutstyle.Apply(baseline, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if astSnapshot(t, baseline) != astSnapshot(t, extended) {
		t.Fatal("extension changed base AST")
	}
}

func TestGofumptExtras(t *testing.T) {
	cfg := noRules()
	cfg.Base.Formatter, cfg.Base.AllowSyntaxChanges = "gofumpt", true
	cfg.Base.Gofumpt.Extra.GroupParams = true
	cfg.Base.Gofumpt.Extra.ClotheReturns = true
	cfg.Base.Gofumpt.Extra.BalanceCalls = true
	got, err := Format([]byte("package p\nfunc f(a string, b string) (err error) { call(\n a,\n b); return }"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("a, b string")) || !bytes.Contains(got, []byte("return err")) || !bytes.Contains(got, []byte("b,\n\t)")) {
		t.Fatalf("extra rules lost:\n%s", got)
	}
}

func TestCLIConfigAndOverrides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yaml")
	if err := os.WriteFile(path, []byte("layout: {max-line-length: 200}\nrules: {wrap-call-arguments: true}\ncalls: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	input := "package p\nfunc f() { allocate(request.TransactionIdentifier, request.PipelineIdentifier, request.ActorIdentifier); audit.Record(\"event\") }"
	var out, stderr bytes.Buffer
	if code := run([]string{"-config", path}, strings.NewReader(input), &out, &stderr); code != 0 {
		t.Fatalf("config exit %d: %s", code, &stderr)
	}
	if bytes.Contains(out.Bytes(), []byte("allocate(\n")) {
		t.Fatal("implicit CLI default overrode YAML")
	}
	out.Reset()
	if code := run([]string{"-config", path, "-m", "80", "-log-methods", "Record"}, strings.NewReader(input), &out, &stderr); code != 0 {
		t.Fatalf("override exit %d: %s", code, &stderr)
	}
	if !bytes.Contains(out.Bytes(), []byte("allocate(\n")) || !bytes.Contains(out.Bytes(), []byte("\n\n\taudit.Record")) {
		t.Fatalf("explicit override lost:\n%s", &out)
	}
	if code := run([]string{"-config", filepath.Join(dir, "missing")}, strings.NewReader(input), &out, &stderr); code != 2 {
		t.Fatal("missing config accepted")
	}
}

func TestCLIDefaultConfig(t *testing.T) {
	userDirectory := t.TempDir()
	t.Setenv("HOME", userDirectory)
	path := filepath.Join(userDirectory, ".config", "mygofmt.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("calls: []"), 0600); err != nil {
		t.Fatal(err)
	}
	input := "package p\nfunc f() { prepare(); log.Info(\"event\"); finish() }"
	var out, stderr bytes.Buffer
	if code := run(nil, strings.NewReader(input), &out, &stderr); code != 0 {
		t.Fatalf("default config exit=%d: %s", code, &stderr)
	}
	if bytes.Contains(out.Bytes(), []byte("prepare()\n\n\tlog.Info")) {
		t.Fatalf("default YAML not applied:\n%s", &out)
	}
	out.Reset()
	if code := run([]string{"-log-methods", "Info"}, strings.NewReader(input), &out, &stderr); code != 0 {
		t.Fatalf("override exit=%d: %s", code, &stderr)
	}
	if !bytes.Contains(out.Bytes(), []byte("prepare()\n\n\tlog.Info")) {
		t.Fatalf("CLI did not override default YAML:\n%s", &out)
	}
}

func TestExampleConfigs(t *testing.T) {
	for _, path := range []string{"mygofmt.yaml.example"} {
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		input, err := os.ReadFile("testdata/02_log_boundaries.input.go")
		if err != nil {
			t.Fatal(err)
		}
		output, err := Format(input, cfg)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Format(output, cfg)
		if err != nil || !bytes.Equal(output, again) {
			t.Fatal("example config is unstable")
		}
	}
}

func FuzzConfiguredFormat(f *testing.F) {
	paths, _ := filepath.Glob("testdata/*.input.go")
	for _, path := range paths {
		input, _ := os.ReadFile(path)
		f.Add(input, uint32(0))
	}
	f.Fuzz(func(t *testing.T, source []byte, mask uint32) {
		if _, err := parser.ParseFile(token.NewFileSet(), "", source, parser.ParseComments); err != nil {
			return
		}
		cfg := testOptions()
		cfg.Layout.Wrap = config.Wrap{
			FunctionParameters: mask&1 == 0, FunctionResults: mask&2 == 0, TypeParameters: mask&4 == 0,
			CallArguments: mask&8 == 0, CompositeLiterals: mask&16 == 0, LogicalConditions: mask&32 == 0,
			NestedCompositeLiterals: mask&131072 == 0,
		}
		cfg.Whitespace = config.Whitespace{
			PreserveExisting: mask&64 == 0, BeforeReturn: mask&128 == 0, BeforeLoop: mask&256 == 0,
			AfterLoop: mask&512 == 0, AfterControlBlock: mask&1024 == 0,
			AfterMultilineInitialization: mask&2048 == 0, CallErrorCheck: "adjacent",
		}
		if mask&4096 != 0 {
			cfg.Whitespace.CallErrorCheck = "preserve"
		}
		if mask&8192 != 0 {
			cfg.Layout.DeferFunctionBody = "preserve"
		}
		cfg.Calls[0].BlankBefore = mask&16384 == 0
		cfg.Calls[0].BlankAfter = mask&32768 == 0
		cfg.Calls[0].IncludeBlockBoundaries = mask&65536 == 0
		output, err := Format(source, cfg)
		if err != nil {
			t.Fatal(err)
		}
		verifyInvariants(t, source, output, cfg)
	})
}

func FuzzGofumptComposition(f *testing.F) {
	paths, _ := filepath.Glob("testdata/*.input.go")
	for _, path := range paths {
		input, _ := os.ReadFile(path)
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, source []byte) {
		if _, err := parser.ParseFile(token.NewFileSet(), "", source, parser.ParseComments); err != nil {
			return
		}
		cfg := testOptions()
		cfg.Base.Formatter, cfg.Base.AllowSyntaxChanges = "gofumpt", true
		output, err := Format(source, cfg)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Format(output, cfg)
		if err != nil || !bytes.Equal(output, again) {
			t.Fatalf("combined pipeline is unstable: %v", err)
		}
		baseline, err := base.Format(source, cfg.Base)
		if err != nil {
			t.Fatal(err)
		}
		extended, err := layoutstyle.Apply(baseline, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if astSnapshot(t, baseline) != astSnapshot(t, extended) {
			t.Fatal("extension changed base AST")
		}
	})
}

func TestConfiguredCallReceivers(t *testing.T) {
	for _, tc := range []struct {
		receiver string
		matched  bool
	}{
		{"fl", true},
		{"s.logger", true},
		{"(fl)", true},
		{"require", false},
		{"t", false},
		{"other.logger", false},
		{"makeLogger()", false},
	} {
		t.Run(tc.receiver, func(t *testing.T) {
			input := []byte("package p\nfunc f() {\nprepare()\n" + tc.receiver + ".Error(\"event\")\nfinish()\n}\n")
			cfg := testOptions()
			cfg.Calls = []config.CallRule{{Methods: []string{"Error"}, Receivers: []string{"fl", "s.logger"}, BlankBefore: true, BlankAfter: true}}
			got, err := Format(input, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if spaced := bytes.Contains(got, []byte("prepare()\n\n")); spaced != tc.matched {
				t.Fatalf("receiver match=%v, want %v:\n%s", spaced, tc.matched, got)
			}
			verifyInvariants(t, input, got, cfg)
		})
	}
}

func TestExampleLoggingProfile(t *testing.T) {
	cfg, err := config.Parse([]byte(`version: 1
base:
  formatter: gofumpt
  allow-syntax-changes: true
rules:
  wrap-function-parameters: false
  wrap-function-results: false
  wrap-type-parameters: false
  wrap-call-arguments: false
  wrap-nested-composite-literals: true
calls:
  - methods: [Trace, Tracef, Debug, Debugf, Info, Infof, Warn, Warnf, Warning, Warningf, Error, Errorf, Fatal, Fatalf, Panic, Panicf, Print, Printf, Println, End, EndElapsed, EndError]
    receivers: [log, logger, fl, s.logger]
    blank-before: true
    blank-after: true
    include-block-boundaries: false
`))
	if err != nil {
		t.Fatal(err)
	}
	input := []byte(`package p
func f() {
    defer func() { fl.EndError("err", err) }()
    prepare()
    require.Error(t, err)
    t.Fatal("test failure")
    s.logger.Infof("event")
    finish()
}
`)
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, unwanted := range []string{"{\n\n\t\tfl.EndError", "fl.EndError(\"err\", err)\n\n", "prepare()\n\n\trequire.Error", "require.Error(t, err)\n\n\tt.Fatal"} {
		if bytes.Contains(got, []byte(unwanted)) {
			t.Errorf("unwanted log gap %q:\n%s", unwanted, got)
		}
	}
	if !bytes.Contains(got, []byte("\n\n\ts.logger.Infof(\"event\")\n\n")) {
		t.Fatalf("logger spacing missing:\n%s", got)
	}
	verifyInvariants(t, input, got, cfg)
}

func TestNestedCompositeSwitch(t *testing.T) {
	input := []byte(`package p
func f() {
    req := Request{
        Context: Context{UserData: map[string]string{"request_id": "id"}},
    }
    use(req)
}

`)
	for _, enabled := range []bool{false, true} {
		cfg := testOptions()
		cfg.Layout.MaxLineLength = 120
		cfg.Layout.Wrap.NestedCompositeLiterals = enabled
		got, err := Format(input, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if wrapped := bytes.Contains(got, []byte("Context: Context{\n")); wrapped != enabled {
			t.Fatalf("nested wrap=%v, want %v:\n%s", wrapped, enabled, got)
		}
		if wrapped := bytes.Contains(got, []byte("UserData: map[string]string{\n")); wrapped != enabled {
			t.Fatalf("map wrap=%v, want %v:\n%s", wrapped, enabled, got)
		}
		verifyInvariants(t, input, got, cfg)
	}
	cfg := testOptions()
	cfg.Layout.MaxLineLength = 120
	cfg.Layout.Wrap.CompositeLiterals = false
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("Context{UserData: map[string]string{\"request_id\": \"id\"}}")) {
		t.Fatalf("master literal switch ignored:\n%s", got)
	}
}

func TestPositionalLiteralWithTrailingCommentsStaysCompact(t *testing.T) {
	input := []byte(`package p
func f() {
	tags := []*models.Tag{
		{Key: &nameKey},    // Name
		{Key: &versionKey}, // ngw-version
	}
	_ = tags
}
`)
	cfg, err := config.Load("mygofmt.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"{Key: &nameKey},    // Name",
		"{Key: &versionKey}, // ngw-version",
	} {
		if !bytes.Contains(got, []byte(line)) {
			t.Errorf("short positional literal expanded: %s\n%s", line, got)
		}
	}
	again, err := Format(got, cfg)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("positional literal formatting is unstable: %v\n%s", err, got)
	}
}

func TestNestedCompositeValues(t *testing.T) {
	for _, value := range []string{
		`&Context{UserData: map[string]string{"id": "value"}}`,
		`(Context{UserData: map[string]string{"id": "value"}})`,
		`map[string]Context{"one": {UserData: map[string]string{"id": "value"}}}`,
	} {
		input := []byte("package p\nfunc f() {\nr := Request{\nContext: " + value + ",\nHosts: []string{\"host-a\"},\nEmpty: Context{},\n}\nuse(r)\n}\n")
		cfg := testOptions()
		cfg.Layout.MaxLineLength = 120
		got, err := Format(input, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(got, []byte("UserData: map[string]string{\n")) {
			t.Fatalf("nested map not wrapped:\n%s", got)
		}
		if !bytes.Contains(got, []byte(`[]string{"host-a"}`)) || !bytes.Contains(got, []byte("Context{}")) {
			t.Fatalf("compact leaf values expanded:\n%s", got)
		}
		verifyInvariants(t, input, got, cfg)
	}
	// A short single-line outer literal stays compact.
	input := []byte("package p\nvar r = Request{Context: Context{Id: id}}\n")
	cfg := testOptions()
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("Request{Context: Context{Id: id}}")) {
		t.Fatalf("short outer literal expanded:\n%s", got)
	}
	verifyInvariants(t, input, got, cfg)
}

func TestDefaultPreservesSingleLineSignatures(t *testing.T) {
	input := []byte(`package p
func validateMutation(ctx common.Context, actor, key, id string, hosts []string, timeout, waitTimeout time.Duration) error { return nil }
func (p *Plugin) BuildReserveItemsRequest(ctx context.Context, req sampletypes.ReserveItemsRequest) (resulttypes.ReserveItemsRequest, error) { panic(0) }
func convert[Request ExampleServiceRequestConstraint, Response ExampleServiceResponseConstraint](request Request) Response { panic(0) }
`)
	cfg := config.Default()
	cfg.Layout.MaxLineLength = 40
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, signature := range []string{
		"func validateMutation(ctx common.Context, actor, key, id string, hosts []string, timeout, waitTimeout time.Duration) error",
		"func (p *Plugin) BuildReserveItemsRequest(ctx context.Context, req sampletypes.ReserveItemsRequest) (resulttypes.ReserveItemsRequest, error)",
		"func convert[Request ExampleServiceRequestConstraint, Response ExampleServiceResponseConstraint](request Request) Response",
	} {
		if !bytes.Contains(got, []byte(signature)) {
			t.Errorf("signature was wrapped: %s\n%s", signature, got)
		}
	}
	verifyInvariants(t, input, got, cfg)
}

func TestGofumptPreservesExistingSignatureLines(t *testing.T) {
	input := []byte(`package p
func (vw *exampleWorker) itemAllocations(ctx context.Context, logger log.Logger,
	commonContext common.Context, key, actor string) (allocations []sampletypes.ItemAllocation, err error) { return }
type Processor interface {
	Reserve(ctx context.Context,
		actor string) error
}
var callback = func(ctx context.Context,
	actor string) error { return nil }
`)
	cfg, err := config.Load("mygofmt.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, signature := range []string{
		"logger log.Logger,\n\tcommonContext common.Context, key, actor string) (allocations []sampletypes.ItemAllocation, err error)",
		"Reserve(ctx context.Context,\n\t\tactor string) error",
		"func(ctx context.Context,\n\tactor string) error",
	} {
		if !bytes.Contains(got, []byte(signature)) {
			t.Errorf("signature layout changed: %s\n%s", signature, got)
		}
	}
	again, err := Format(got, cfg)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("signature formatting is unstable: %v\n%s", err, got)
	}
}

func TestGofumptKeepsBlockCommentBoundary(t *testing.T) {
	input := []byte(`package p
func f() {
	taskID, err := start()

	/*
		options := &example.Options{ID: "sample"}
		taskID, err := service.Start(options)
	*/

	if err != nil {
		return
	}
	_ = taskID
}
`)
	cfg, err := config.Load("mygofmt.yaml.example")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("taskID, err := start()\n\n\t/*")) || !bytes.Contains(got, []byte("\t*/\n\n\tif err != nil")) {
		t.Fatalf("block comment boundary changed:\n%s", got)
	}
	again, err := Format(got, cfg)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("block comment formatting is unstable: %v\n%s", err, got)
	}
}

func TestMinimumCallArguments(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		minimum          int
		wrapped          bool
	}{
		{"zero", "allocate()", 2, false},
		{"one", "allocate(request.VeryLongTransactionIdentifier)", 2, false},
		{"variadic one", "allocate(request.AdditionalArguments...)", 2, false},
		{"two", "allocate(request.VeryLongTransactionIdentifier, request.VeryLongPipelineIdentifier)", 2, true},
		{"explicit minimum one", "allocate(request.VeryLongTransactionIdentifier)", 1, true},
		{"explicit minimum three", "allocate(request.VeryLongTransactionIdentifier, request.VeryLongPipelineIdentifier)", 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Layout.MaxLineLength = 30
			cfg.Layout.MinCallArguments = tc.minimum
			cfg.Layout.Wrap.CallArguments = true
			input := []byte("package p\nfunc f() {\n" + tc.expression + "\n}\n")
			got, err := Format(input, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if wrapped := bytes.Contains(got, []byte("allocate(\n")); wrapped != tc.wrapped {
				t.Fatalf("wrapped=%v, want %v:\n%s", wrapped, tc.wrapped, got)
			}
			verifyInvariants(t, input, got, cfg)
		})
	}
}

func TestSingleArgumentCallWithLiteralReceiver(t *testing.T) {
	input := []byte(`package p
func f() {
    multiplier, err := (sampletypes.Config{PluginExampleConfig: cfg}).ReservationMultiplier(sampletypes.ResourceTypeOther)
    use(multiplier, err)
}
`)
	cfg := config.Default()
	cfg.Layout.MaxLineLength = 80
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("(sampletypes.Config{PluginExampleConfig: cfg}).ReservationMultiplier(sampletypes.ResourceTypeOther)")) {
		t.Fatalf("single argument call or short receiver literal was wrapped:\n%s", got)
	}
	verifyInvariants(t, input, got, cfg)
}

func TestDefaultDoesNotWrapCalls(t *testing.T) {
	input := []byte(`package p
func f() {
    s.logger.Infof("select item: item_id=%s selected_item=%s remaining_items=%v", req.ItemId, selected.Name, remainingItems)
    allocate(request.VeryLongTransactionIdentifier, request.VeryLongPipelineIdentifier, request.VeryLongActorIdentifier)
    manuallyWrapped(
        first,
        second,
    )
}
`)
	cfg := config.Default()
	cfg.Layout.MaxLineLength = 40
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{
		`s.logger.Infof("select item: item_id=%s selected_item=%s remaining_items=%v", req.ItemId, selected.Name, remainingItems)`,
		`allocate(request.VeryLongTransactionIdentifier, request.VeryLongPipelineIdentifier, request.VeryLongActorIdentifier)`,
		"manuallyWrapped(\n",
	} {
		if !bytes.Contains(got, []byte(call)) {
			t.Errorf("call layout changed: %s\n%s", call, got)
		}
	}
	verifyInvariants(t, input, got, cfg)
}

func TestBalancedConditionLines(t *testing.T) {
	for _, expression := range []string{
		`failure.Causes[0].ServerError != first || failure.Causes[1].ServerError != second || failure.Causes[0].Message != "" || failure.Causes[1].Message != ""`,
		"failure.Causes[0].ServerError != first || failure.Causes[1].ServerError != second ||\n failure.Causes[0].Message != \"\" ||\n failure.Causes[1].Message != \"\"",
	} {
		input := []byte("package p\nfunc f() {\nif " + expression + " { run() }\n}\n")
		cfg := config.Default()
		got, err := Format(input, cfg)
		if err != nil {
			t.Fatal(err)
		}
		want := "if failure.Causes[0].ServerError != first || failure.Causes[1].ServerError != second ||\n\t\tfailure.Causes[0].Message != \"\" || failure.Causes[1].Message != \"\" {"
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("condition was not balanced across two lines:\n%s", got)
		}
		verifyInvariants(t, input, got, cfg)
	}
}

func TestBalancedConditionBoundaries(t *testing.T) {
	cases := []struct {
		name, expression string
		limit, lines     int
	}{
		{"short", "a || b || c", 120, 1},
		{"short previously wrapped", "a ||\n b ||\n c", 120, 1},
		{"three lines", "firstLongIdentifier || secondLongIdentifier || thirdLongIdentifier", 32, 3},
		{"parentheses and precedence", "firstLongIdentifier && (secondLongIdentifier || thirdLongIdentifier) || fourthLongIdentifier", 80, 2},
		{"mixed operators", "firstLongIdentifier || secondLongIdentifier && thirdLongIdentifier || fourthLongIdentifier", 80, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte("package p\nfunc f() {\nif " + tc.expression + " { run() }\n}\n")
			cfg := config.Default()
			cfg.Layout.MaxLineLength = tc.limit
			got, err := Format(input, cfg)
			if err != nil {
				t.Fatal(err)
			}
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, "", got, 0)
			if err != nil {
				t.Fatal(err)
			}
			condition := file.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.IfStmt).Cond
			lines := fileSet.Position(condition.End()).Line - fileSet.Position(condition.Pos()).Line + 1
			if lines != tc.lines {
				t.Fatalf("condition lines=%d, want %d:\n%s", lines, tc.lines, got)
			}
			verifyInvariants(t, input, got, cfg)
		})
	}
}

func TestBalancedConditionsPreserveCommentsAndRawStrings(t *testing.T) {
	input := []byte("package p\nfunc f() {\nif firstLongIdentifier || // keep this comment\nsecondLongIdentifier { run() }\nif check(`first\nsecond`) || secondLongIdentifier { run() }\n}\n")
	cfg := config.Default()
	cfg.Layout.MaxLineLength = 30
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	verifyInvariants(t, input, got, cfg)
	cfg.Layout.Wrap.LogicalConditions = false
	input = []byte("package p\nfunc f() {\nif a ||\n b ||\n c { run() }\n}\n")
	got, err = Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("if a ||\n")) {
		t.Fatalf("disabled condition rule changed layout:\n%s", got)
	}
	verifyInvariants(t, input, got, cfg)
}

func TestSingleStatementBlockHasNoInnerBlankLines(t *testing.T) {
	input := []byte(`package p
func f(ok bool) {
    if ok {

        t.Fatalf("result item_id = %v, want item-a", result["item_id"])

    }
    if ok {

        // Keep this comment with the statement.
        t.Fatal("failure")

    }
    defer func() {

        log.Info("done")

    }()
}
`)
	cfg := config.Default()
	got, err := Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("if ok {\n\t\tt.Fatalf")) ||
		!bytes.Contains(got, []byte("t.Fatalf(\"result item_id = %v, want item-a\", result[\"item_id\"])\n\t}")) ||
		!bytes.Contains(got, []byte("defer func() {\n\t\tlog.Info(\"done\")\n\t}()")) {
		t.Fatalf("one-statement block contains empty lines:\n%s", got)
	}
	if !bytes.Contains(got, []byte("// Keep this comment with the statement.")) {
		t.Fatalf("comment was lost:\n%s", got)
	}
	verifyInvariants(t, input, got, cfg)
	cfg.Whitespace.PreserveExisting = false
	got, err = Format(input, cfg)
	if err != nil {
		t.Fatal(err)
	}
	verifyInvariants(t, input, got, cfg)
}
