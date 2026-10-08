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
	}
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
	cfg := Default()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return Config{}, fmt.Errorf("config: %w", err)
		}
		return Config{}, fmt.Errorf("config must contain exactly one YAML document")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
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
