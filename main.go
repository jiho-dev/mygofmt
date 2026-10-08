// mygofmt formats Go source using the configurable whitespace conventions.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jiho-dev/mygofmt/internal/config"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("mygofmt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	write := flags.Bool("w", false, "write formatted source to files")
	check := flags.Bool("check", false, "list files needing formatting; exit 1 if any")
	list := flags.Bool("l", false, "list files needing formatting")
	configPath := flags.String("config", "", "YAML configuration file (default ~/.config/mygofmt.yaml; built-in defaults if absent)")
	maxLine := flags.Int("m", 120, "preferred maximum line width (not a hard limit)")
	tabWidth := flags.Int("tab-width", 4, "display width of each tab")
	logNames := flags.String("log-methods", config.DefaultLogMethods, "comma-separated logging method names")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *write && *check {
		fmt.Fprintln(stderr, "mygofmt: -w and -check cannot be combined")
		return 2
	}
	options, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, "mygofmt:", err)
		return 2
	}
	flags.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "m":
			options.Layout.MaxLineLength = *maxLine
		case "tab-width":
			options.Layout.TabWidth = *tabWidth
		case "log-methods":
			options.Calls = nil
			if *logNames != "" {
				methods := strings.Split(*logNames, ",")
				for i := range methods {
					methods[i] = strings.TrimSpace(methods[i])
				}
				options.Calls = []config.CallRule{{Methods: methods, BlankBefore: true, BlankAfter: true, IncludeBlockBoundaries: true}}
			}
		}
	})
	if err := options.Validate(); err != nil {
		fmt.Fprintln(stderr, "mygofmt:", err)
		return 2
	}
	if flags.NArg() == 0 {
		if *write {
			fmt.Fprintln(stderr, "mygofmt: -w requires a file or directory")
			return 2
		}
		input, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintln(stderr, "mygofmt:", err)
			return 2
		}
		output, err := Format(input, options)
		if err != nil {
			fmt.Fprintln(stderr, "mygofmt:", err)
			return 2
		}
		if *check || *list {
			if !bytes.Equal(input, output) {
				fmt.Fprintln(stdout, "<stdin>")
				if *check {
					return 1
				}
			}
			return 0
		}
		if _, err := stdout.Write(output); err != nil {
			fmt.Fprintln(stderr, "mygofmt:", err)
			return 2
		}
		return 0
	}
	paths, err := collectFiles(flags.Args())
	if err != nil {
		fmt.Fprintln(stderr, "mygofmt:", err)
		return 2
	}
	// Parse every input before writing anything, so syntax errors cannot produce
	// a partially formatted batch.
	type result struct {
		path          string
		input, output []byte
	}
	results := make([]result, 0, len(paths))
	for _, path := range paths {
		input, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, "mygofmt:", err)
			return 2
		}
		output, err := Format(input, options)
		if err != nil {
			fmt.Fprintf(stderr, "mygofmt: %s: %v\n", path, err)
			return 2
		}
		results = append(results, result{path, input, output})
	}
	exitCode := 0
	for _, item := range results {
		changed := !bytes.Equal(item.input, item.output)
		if changed && (*list || *check) {
			fmt.Fprintln(stdout, item.path)
		}
		if changed && *check {
			exitCode = 1
		}
		if changed && *write {
			info, err := os.Stat(item.path)
			if err == nil {
				err = os.WriteFile(item.path, item.output, info.Mode().Perm())
			}
			if err != nil {
				fmt.Fprintln(stderr, "mygofmt:", err)
				return 2
			}
		}
		if !*write && !*list && !*check {
			if _, err := stdout.Write(item.output); err != nil {
				fmt.Fprintln(stderr, "mygofmt:", err)
				return 2
			}
		}
	}
	return exitCode
}

func collectFiles(inputs []string) ([]string, error) {
	seen := make(map[string]bool)
	for _, input := range inputs {
		err := filepath.WalkDir(input, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				if path != input {
					return nil
				}
				return fmt.Errorf("symbolic links are not supported: %s", path)
			}
			if entry.IsDir() {
				if path != input && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata") {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type().IsRegular() && strings.HasSuffix(path, ".go") {
				seen[filepath.Clean(path)] = true
			} else if path == input {
				return fmt.Errorf("expected a Go file or directory: %s", path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}
