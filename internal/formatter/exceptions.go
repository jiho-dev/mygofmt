package formatter

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

type exceptionDirective struct {
	kind       string
	text       string
	start      int
	lineStart  int
	lineEnd    int
	lineNumber int
}

type protectedRegion struct {
	begin, end int
	marker     string
	original   []byte
}

type exceptionPlan struct {
	source    []byte
	skipFile  bool
	kinds     []string
	protected []protectedRegion
}

func parseExceptions(source []byte, fset *token.FileSet, file *ast.File) (exceptionPlan, error) {
	plan := exceptionPlan{source: source}
	directives, err := exceptionDirectives(source, fset, file, false)
	if err != nil {
		return plan, err
	}
	if len(directives) == 0 {
		return plan, nil
	}
	for _, directive := range directives {
		plan.kinds = append(plan.kinds, directive.kind)
	}
	for _, directive := range directives {
		if directive.kind != "ignore-file" {
			continue
		}
		if len(directives) != 1 || directive.lineStart >= fset.PositionFor(file.Package, false).Offset {
			return plan, fmt.Errorf("mygofmt:ignore-file must be the only directive before package")
		}
		plan.skipFile = true
		return plan, nil
	}
	type insertion struct {
		pos  int
		text []byte
	}
	var insertions []insertion
	open := -1
	for i, directive := range directives {
		switch directive.kind {
		case "off":
			if open >= 0 {
				return plan, fmt.Errorf("mygofmt:off at line %d is nested", directive.lineNumber)
			}
			open = i
		case "on":
			if open < 0 {
				return plan, fmt.Errorf("mygofmt:on at line %d has no matching off", directive.lineNumber)
			}
			plan.protected = append(plan.protected, protectedRegion{
				begin: open, end: i,
				original: bytes.Clone(source[directives[open].lineEnd:directive.lineStart]),
			})
			open = -1
		case "ignore":
			if open >= 0 {
				return plan, fmt.Errorf("mygofmt:ignore at line %d is inside an off/on region", directive.lineNumber)
			}
			start := directive.lineEnd
			if start >= len(source) {
				return plan, fmt.Errorf("mygofmt:ignore at line %d has no next line", directive.lineNumber)
			}
			end := sourceLineEnd(source, start)
			if len(bytes.TrimSpace(source[start:end])) == 0 || (i+1 < len(directives) && directives[i+1].lineStart == start) {
				return plan, fmt.Errorf("mygofmt:ignore at line %d needs a nonempty source line", directive.lineNumber)
			}
			marker := fmt.Sprintf("// mygofmt:__internal-line-end:%d", i)
			indent := source[directive.lineStart:directive.start]
			// The source may have no final newline; the marker still needs its own line.
			prefix := ""
			if end == len(source) && source[end-1] != '\n' {
				prefix = "\n"
			}
			insertions = append(insertions, insertion{end, []byte(prefix + string(indent) + marker + "\n")})
			plan.protected = append(plan.protected, protectedRegion{
				begin: i, marker: marker, original: bytes.Clone(source[start:end]),
			})
		}
	}
	if open >= 0 {
		return plan, fmt.Errorf("mygofmt:off at line %d has no matching on", directives[open].lineNumber)
	}
	for i := len(insertions) - 1; i >= 0; i-- {
		insertion := insertions[i]
		plan.source = append(append(bytes.Clone(plan.source[:insertion.pos]), insertion.text...), plan.source[insertion.pos:]...)
	}
	return plan, nil
}

func (p exceptionPlan) restore(formatted []byte) ([]byte, error) {
	if len(p.protected) == 0 {
		return formatted, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", formatted, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	directives, err := exceptionDirectives(formatted, fset, file, true)
	if err != nil {
		return nil, err
	}
	var users []exceptionDirective
	markers := make(map[string]exceptionDirective)
	for _, directive := range directives {
		if directive.kind == "internal" {
			markers[directive.text] = directive
		} else {
			users = append(users, directive)
		}
	}
	if len(users) != len(p.kinds) {
		return nil, fmt.Errorf("mygofmt exception directives changed during formatting")
	}
	for i, directive := range users {
		if directive.kind != p.kinds[i] {
			return nil, fmt.Errorf("mygofmt exception directives changed order during formatting")
		}
	}
	type edit struct {
		start, end int
		text       []byte
	}
	var edits []edit
	for _, region := range p.protected {
		start := users[region.begin].lineEnd
		end := 0
		if region.marker != "" {
			marker, ok := markers[region.marker]
			if !ok {
				return nil, fmt.Errorf("mygofmt exception line marker was lost during formatting")
			}
			end = marker.lineEnd
		} else {
			end = users[region.end].lineStart
		}
		if end < start {
			return nil, fmt.Errorf("mygofmt exception region moved during formatting")
		}
		edits = append(edits, edit{start, end, region.original})
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var output bytes.Buffer
	last := 0
	for _, edit := range edits {
		if edit.start < last {
			return nil, fmt.Errorf("mygofmt exception regions overlap after formatting")
		}
		output.Write(formatted[last:edit.start])
		output.Write(edit.text)
		last = edit.end
	}
	output.Write(formatted[last:])
	result := output.Bytes()
	if _, err := parser.ParseFile(token.NewFileSet(), "", result, parser.ParseComments); err != nil {
		return nil, fmt.Errorf("mygofmt exception restoration: %w", err)
	}
	return result, nil
}

func exceptionDirectives(source []byte, fset *token.FileSet, file *ast.File, allowInternal bool) ([]exceptionDirective, error) {
	var directives []exceptionDirective
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if !strings.HasPrefix(comment.Text, "//") {
				continue
			}
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if !strings.HasPrefix(text, "mygofmt:") {
				continue
			}
			kind := strings.TrimPrefix(text, "mygofmt:")
			if strings.HasPrefix(kind, "__internal-line-end:") && allowInternal {
				kind = "internal"
			} else if kind != "ignore" && kind != "off" && kind != "on" && kind != "ignore-file" {
				return nil, fmt.Errorf("unknown mygofmt directive %q", text)
			}
			start := fset.PositionFor(comment.Pos(), false).Offset
			lineStart := sourceLineStart(source, start)
			if len(bytes.TrimSpace(source[lineStart:start])) != 0 {
				return nil, fmt.Errorf("mygofmt directive at line %d must be on its own line", fset.PositionFor(comment.Pos(), false).Line)
			}
			directives = append(directives, exceptionDirective{
				kind: kind, text: comment.Text, start: start, lineStart: lineStart,
				lineEnd: sourceLineEnd(source, start), lineNumber: fset.PositionFor(comment.Pos(), false).Line,
			})
		}
	}
	return directives, nil
}

func sourceLineStart(source []byte, offset int) int {
	return bytes.LastIndexByte(source[:offset], '\n') + 1
}

func sourceLineEnd(source []byte, offset int) int {
	if end := bytes.IndexByte(source[offset:], '\n'); end >= 0 {
		return offset + end + 1
	}
	return len(source)
}
