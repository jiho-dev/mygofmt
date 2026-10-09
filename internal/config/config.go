// Package config defines the declarative formatting policy.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"go/version"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultLogMethods = "Trace,Tracef,Debug,Debugf,Info,Infof,Warn,Warnf,Warning,Warningf,Error,Errorf,Fatal,Fatalf,Panic,Panicf,Print,Printf,Println,End,EndElapsed,EndError"

type Config struct {
	Version    int        `yaml:"version"`
	Base       Base       `yaml:"base"`
	Layout     Layout     `yaml:"layout"`
	Whitespace Whitespace `yaml:"whitespace"`
	Calls      []CallRule `yaml:"calls"`
	Files      Files      `yaml:"files"`
	Rules      Rules      `yaml:"rules"`
}

// Rules contains named switches for the formatter's independently selectable
// behaviors. Parameter values and input-file policies remain in their own
// config sections.
type Rules struct {
	WrapFunctionParameters       bool `yaml:"wrap-function-parameters"`
	WrapFunctionResults          bool `yaml:"wrap-function-results"`
	WrapTypeParameters           bool `yaml:"wrap-type-parameters"`
	WrapCallArguments            bool `yaml:"wrap-call-arguments"`
	WrapCompositeLiterals        bool `yaml:"wrap-composite-literals"`
	WrapNestedCompositeLiterals  bool `yaml:"wrap-nested-composite-literals"`
	BalanceLogicalConditions     bool `yaml:"balance-logical-conditions"`
	MultilineDeferBody           bool `yaml:"multiline-defer-body"`
	PreserveExistingBlankLines   bool `yaml:"preserve-existing-blank-lines"`
	BlankBeforeReturn            bool `yaml:"blank-before-return"`
	BlankBeforeLoop              bool `yaml:"blank-before-loop"`
	BlankAfterLoop               bool `yaml:"blank-after-loop"`
	BlankAfterControlBlock       bool `yaml:"blank-after-control-block"`
	BlankAfterMultilineInit      bool `yaml:"blank-after-multiline-initialization"`
	JoinCallErrorChecks          bool `yaml:"join-call-error-checks"`
	SpaceConfiguredCalls         bool `yaml:"space-configured-calls"`
	CompactSingleStatementBlocks bool `yaml:"compact-single-statement-blocks"`
	PreserveBlockCommentGaps     bool `yaml:"preserve-block-comment-gaps"`
	PreserveSignatureLayout      bool `yaml:"preserve-signature-layout"`
	GofumptGroupParams           bool `yaml:"gofumpt-group-params"`
	GofumptClotheReturns         bool `yaml:"gofumpt-clothe-returns"`
	GofumptBalanceCalls          bool `yaml:"gofumpt-balance-calls"`
}

// yamlConfig is the public YAML shape. Legacy rule switches in base, layout,
// and whitespace are deliberately absent, so KnownFields reports them.
type yamlConfig struct {
	Version int `yaml:"version"`
	Base    struct {
		Formatter          string `yaml:"formatter"`
		AllowSyntaxChanges bool   `yaml:"allow-syntax-changes"`
		Gofumpt            struct {
			LangVersion string `yaml:"lang-version"`
			ModulePath  string `yaml:"module-path"`
		} `yaml:"gofumpt"`
	} `yaml:"base"`
	Layout struct {
		MaxLineLength    int `yaml:"max-line-length"`
		TabWidth         int `yaml:"tab-width"`
		MinCallArguments int `yaml:"min-call-arguments"`
	} `yaml:"layout"`
	Calls []CallRule `yaml:"calls"`
	Files Files      `yaml:"files"`
	Rules Rules      `yaml:"rules"`
}

type Base struct {
	Formatter          string  `yaml:"formatter"`
	AllowSyntaxChanges bool    `yaml:"allow-syntax-changes"`
	Gofumpt            Gofumpt `yaml:"gofumpt"`
}

type Gofumpt struct {
	LangVersion string `yaml:"lang-version"`
	ModulePath  string `yaml:"module-path"`
	Extra       Extra  `yaml:"extra"`
}

type Extra struct {
	GroupParams   bool `yaml:"group-params"`
	ClotheReturns bool `yaml:"clothe-returns"`
	BalanceCalls  bool `yaml:"balance-calls"`
}

type Layout struct {
	MaxLineLength     int    `yaml:"max-line-length"`
	TabWidth          int    `yaml:"tab-width"`
	MinCallArguments  int    `yaml:"min-call-arguments"`
	Wrap              Wrap   `yaml:"wrap"`
	DeferFunctionBody string `yaml:"defer-function-body"`
}

type Wrap struct {
	FunctionParameters      bool `yaml:"function-parameters"`
	FunctionResults         bool `yaml:"function-results"`
	TypeParameters          bool `yaml:"type-parameters"`
	CallArguments           bool `yaml:"call-arguments"`
	CompositeLiterals       bool `yaml:"composite-literals"`
	NestedCompositeLiterals bool `yaml:"nested-composite-literals"`
	LogicalConditions       bool `yaml:"logical-conditions"`
}

type Whitespace struct {
	PreserveExisting             bool   `yaml:"preserve-existing"`
	BeforeReturn                 bool   `yaml:"before-return"`
	BeforeLoop                   bool   `yaml:"before-loop"`
	AfterLoop                    bool   `yaml:"after-loop"`
	AfterControlBlock            bool   `yaml:"after-control-block"`
	AfterMultilineInitialization bool   `yaml:"after-multiline-initialization"`
	CallErrorCheck               string `yaml:"call-error-check"`
}

type CallRule struct {
	Methods                []string `yaml:"methods"`
	Receivers              []string `yaml:"receivers"`
	BlankBefore            bool     `yaml:"blank-before"`
	BlankAfter             bool     `yaml:"blank-after"`
	IncludeBlockBoundaries bool     `yaml:"include-block-boundaries"`
}

type Files struct {
	Generated      string `yaml:"generated"`
	LineDirectives string `yaml:"line-directives"`
}

func Default() Config {
	return Config{
		Version: 1,
		Base:    Base{Formatter: "gofmt", Gofumpt: Gofumpt{LangVersion: "go1.25"}},
		Layout: Layout{MaxLineLength: 120, TabWidth: 4, MinCallArguments: 2, Wrap: Wrap{
			FunctionParameters: false, FunctionResults: false, TypeParameters: false,
			CallArguments: false, CompositeLiterals: true, NestedCompositeLiterals: true, LogicalConditions: true,
		}, DeferFunctionBody: "multiline"},
		Whitespace: Whitespace{true, true, true, true, true, true, "adjacent"},
		Calls:      []CallRule{{Methods: strings.Split(DefaultLogMethods, ","), BlankBefore: true, BlankAfter: true, IncludeBlockBoundaries: true}},
		Files:      Files{Generated: "gofmt-only", LineDirectives: "gofmt-only"},
		Rules: Rules{
			WrapCompositeLiterals: true, WrapNestedCompositeLiterals: true, BalanceLogicalConditions: true,
			MultilineDeferBody: true, PreserveExistingBlankLines: true, BlankBeforeReturn: true,
			BlankBeforeLoop: true, BlankAfterLoop: true, BlankAfterControlBlock: true,
			BlankAfterMultilineInit: true, JoinCallErrorChecks: true, SpaceConfiguredCalls: true,
			CompactSingleStatementBlocks: true, PreserveBlockCommentGaps: true, PreserveSignatureLayout: true,
		},
	}
}

func defaultYAML() yamlConfig {
	return Default().yaml()
}

// MarshalYAML writes the public schema rather than the legacy runtime fields.
func (cfg Config) MarshalYAML() (any, error) {
	return cfg.yaml(), nil
}

func (cfg Config) yaml() yamlConfig {
	var doc yamlConfig
	doc.Version = cfg.Version
	doc.Base.Formatter = cfg.Base.Formatter
	doc.Base.AllowSyntaxChanges = cfg.Base.AllowSyntaxChanges
	doc.Base.Gofumpt.LangVersion = cfg.Base.Gofumpt.LangVersion
	doc.Base.Gofumpt.ModulePath = cfg.Base.Gofumpt.ModulePath
	doc.Layout.MaxLineLength = cfg.Layout.MaxLineLength
	doc.Layout.TabWidth = cfg.Layout.TabWidth
	doc.Layout.MinCallArguments = cfg.Layout.MinCallArguments
	doc.Calls = cfg.Calls
	doc.Files = cfg.Files
	doc.Rules = cfg.Rules
	doc.Rules.WrapFunctionParameters = cfg.Layout.Wrap.FunctionParameters
	doc.Rules.WrapFunctionResults = cfg.Layout.Wrap.FunctionResults
	doc.Rules.WrapTypeParameters = cfg.Layout.Wrap.TypeParameters
	doc.Rules.WrapCallArguments = cfg.Layout.Wrap.CallArguments
	doc.Rules.WrapCompositeLiterals = cfg.Layout.Wrap.CompositeLiterals
	doc.Rules.WrapNestedCompositeLiterals = cfg.Layout.Wrap.NestedCompositeLiterals
	doc.Rules.BalanceLogicalConditions = cfg.Layout.Wrap.LogicalConditions
	doc.Rules.MultilineDeferBody = cfg.Layout.DeferFunctionBody == "multiline"
	doc.Rules.PreserveExistingBlankLines = cfg.Whitespace.PreserveExisting
	doc.Rules.BlankBeforeReturn = cfg.Whitespace.BeforeReturn
	doc.Rules.BlankBeforeLoop = cfg.Whitespace.BeforeLoop
	doc.Rules.BlankAfterLoop = cfg.Whitespace.AfterLoop
	doc.Rules.BlankAfterControlBlock = cfg.Whitespace.AfterControlBlock
	doc.Rules.BlankAfterMultilineInit = cfg.Whitespace.AfterMultilineInitialization
	doc.Rules.JoinCallErrorChecks = cfg.Whitespace.CallErrorCheck == "adjacent"
	doc.Rules.GofumptGroupParams = cfg.Base.Gofumpt.Extra.GroupParams
	doc.Rules.GofumptClotheReturns = cfg.Base.Gofumpt.Extra.ClotheReturns
	doc.Rules.GofumptBalanceCalls = cfg.Base.Gofumpt.Extra.BalanceCalls
	return doc
}

func (doc yamlConfig) config() Config {
	cfg := Default()
	cfg.Version = doc.Version
	cfg.Base.Formatter = doc.Base.Formatter
	cfg.Base.AllowSyntaxChanges = doc.Base.AllowSyntaxChanges
	cfg.Base.Gofumpt.LangVersion = doc.Base.Gofumpt.LangVersion
	cfg.Base.Gofumpt.ModulePath = doc.Base.Gofumpt.ModulePath
	cfg.Layout.MaxLineLength = doc.Layout.MaxLineLength
	cfg.Layout.TabWidth = doc.Layout.TabWidth
	cfg.Layout.MinCallArguments = doc.Layout.MinCallArguments
	cfg.Calls = doc.Calls
	cfg.Files = doc.Files
	cfg.Rules = doc.Rules
	r := doc.Rules
	cfg.Layout.Wrap = Wrap{
		FunctionParameters: r.WrapFunctionParameters, FunctionResults: r.WrapFunctionResults,
		TypeParameters: r.WrapTypeParameters, CallArguments: r.WrapCallArguments,
		CompositeLiterals: r.WrapCompositeLiterals, NestedCompositeLiterals: r.WrapNestedCompositeLiterals,
		LogicalConditions: r.BalanceLogicalConditions,
	}
	if r.MultilineDeferBody {
		cfg.Layout.DeferFunctionBody = "multiline"
	} else {
		cfg.Layout.DeferFunctionBody = "preserve"
	}
	cfg.Whitespace = Whitespace{
		PreserveExisting: r.PreserveExistingBlankLines,
		BeforeReturn:     r.BlankBeforeReturn, BeforeLoop: r.BlankBeforeLoop,
		AfterLoop: r.BlankAfterLoop, AfterControlBlock: r.BlankAfterControlBlock,
		AfterMultilineInitialization: r.BlankAfterMultilineInit,
		CallErrorCheck:               "preserve",
	}
	if r.JoinCallErrorChecks {
		cfg.Whitespace.CallErrorCheck = "adjacent"
	}
	cfg.Base.Gofumpt.Extra = Extra{
		GroupParams:   r.GofumptGroupParams,
		ClotheReturns: r.GofumptClotheReturns,
		BalanceCalls:  r.GofumptBalanceCalls,
	}
	return cfg
}

func Load(path string) (Config, error) {
	useDefaultPath := path == ""
	if path == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve default config: %w", err)
		}
		path = filepath.Join(userHome, ".config", "mygofmt.yaml")
	}
	data, err := os.ReadFile(path)
	if useDefaultPath && errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return Config{}, fmt.Errorf("config must be a YAML mapping")
	}
	if err := rejectNulls(&document); err != nil {
		return Config{}, err
	}
	if err := rejectLegacyRuleKeys(document.Content[0]); err != nil {
		return Config{}, err
	}
	doc := defaultYAML()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
		return Config{}, fmt.Errorf("config must contain exactly one YAML document")
	}
	cfg := doc.config()
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func rejectLegacyRuleKeys(mapping *yaml.Node) error {
	oldWhitespace := map[string]string{
		"preserve-existing":              "preserve-existing-blank-lines",
		"before-return":                  "blank-before-return",
		"before-loop":                    "blank-before-loop",
		"after-loop":                     "blank-after-loop",
		"after-control-block":            "blank-after-control-block",
		"after-multiline-initialization": "blank-after-multiline-initialization",
		"call-error-check":               "join-call-error-checks",
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i].Value, mapping.Content[i+1]
		switch key {
		case "whitespace":
			if len(value.Content) >= 2 {
				old := value.Content[0].Value
				if replacement, ok := oldWhitespace[old]; ok {
					return fmt.Errorf("config line %d: whitespace.%s moved to rules.%s", value.Content[0].Line, old, replacement)
				}
			}
			return fmt.Errorf("config line %d: whitespace rules moved to rules.*", mapping.Content[i].Line)
		case "layout":
			for j := 0; j+1 < len(value.Content); j += 2 {
				switch value.Content[j].Value {
				case "defer-function-body":
					return fmt.Errorf("config line %d: layout.defer-function-body moved to rules.multiline-defer-body", value.Content[j].Line)
				case "wrap":
					wrap := value.Content[j+1]
					if len(wrap.Content) >= 2 {
						old := wrap.Content[0].Value
						newName := "wrap-" + old
						if old == "logical-conditions" {
							newName = "balance-logical-conditions"
						}
						return fmt.Errorf("config line %d: layout.wrap.%s moved to rules.%s", wrap.Content[0].Line, old, newName)
					}
					return fmt.Errorf("config line %d: layout.wrap moved to rules.wrap-*", value.Content[j].Line)
				}
			}
		case "base":
			for j := 0; j+1 < len(value.Content); j += 2 {
				if value.Content[j].Value != "gofumpt" {
					continue
				}
				gofumpt := value.Content[j+1]
				for k := 0; k+1 < len(gofumpt.Content); k += 2 {
					if gofumpt.Content[k].Value == "extra" {
						extra := gofumpt.Content[k+1]
						if len(extra.Content) >= 2 {
							return fmt.Errorf("config line %d: base.gofumpt.extra.%s moved to rules.gofumpt-%s", extra.Content[0].Line, extra.Content[0].Value, extra.Content[0].Value)
						}
						return fmt.Errorf("config line %d: base.gofumpt.extra moved to rules.gofumpt-*", gofumpt.Content[k].Line)
					}
				}
			}
		}
	}
	return nil
}

func rejectNulls(node *yaml.Node) error {
	if node.Tag == "!!null" {
		return fmt.Errorf("config line %d: null values are not supported", node.Line)
	}
	for _, child := range node.Content {
		if err := rejectNulls(child); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) Validate() error {
	if c.Version != 1 {
		return fmt.Errorf("unsupported config version: %d", c.Version)
	}
	if c.Base.Formatter != "gofmt" && c.Base.Formatter != "gofumpt" {
		return fmt.Errorf("base.formatter must be gofmt or gofumpt")
	}
	if c.Base.Formatter == "gofumpt" && !c.Base.AllowSyntaxChanges {
		return fmt.Errorf("gofumpt requires base.allow-syntax-changes: true")
	}
	if !version.IsValid(c.Base.Gofumpt.LangVersion) {
		return fmt.Errorf("base.gofumpt.lang-version must be a valid Go version, e.g. go1.25")
	}
	if c.Layout.MaxLineLength < 20 {
		return fmt.Errorf("layout.max-line-length must be at least 20")
	}
	if c.Layout.TabWidth < 1 {
		return fmt.Errorf("layout.tab-width must be positive")
	}
	if c.Layout.MinCallArguments < 1 {
		return fmt.Errorf("layout.min-call-arguments must be at least 1")
	}
	if c.Layout.DeferFunctionBody != "multiline" && c.Layout.DeferFunctionBody != "preserve" {
		return fmt.Errorf("layout.defer-function-body must be multiline or preserve")
	}
	if c.Whitespace.CallErrorCheck != "adjacent" && c.Whitespace.CallErrorCheck != "preserve" {
		return fmt.Errorf("whitespace.call-error-check must be adjacent or preserve")
	}
	for name, policy := range map[string]string{"generated": c.Files.Generated, "line-directives": c.Files.LineDirectives} {
		if policy != "gofmt-only" && policy != "skip" {
			return fmt.Errorf("files.%s must be gofmt-only or skip", name)
		}
	}
	seen := make(map[string]bool)
	for i, rule := range c.Calls {
		receivers := make(map[string]bool)
		for _, receiver := range rule.Receivers {
			for _, part := range strings.Split(receiver, ".") {
				if !token.IsIdentifier(part) || part == "_" {
					return fmt.Errorf("invalid call receiver: %q", receiver)
				}
			}
			if receivers[receiver] {
				return fmt.Errorf("duplicate call receiver: %s", receiver)
			}
			receivers[receiver] = true
		}
		if len(rule.Methods) == 0 {
			return fmt.Errorf("calls[%d].methods must not be empty (use calls: [] to disable call rules)", i)
		}
		for _, method := range rule.Methods {
			if !token.IsIdentifier(method) {
				return fmt.Errorf("invalid call method: %q", method)
			}
			if seen[method] {
				return fmt.Errorf("duplicate call method: %s", method)
			}
			seen[method] = true
		}
	}
	return nil
}
