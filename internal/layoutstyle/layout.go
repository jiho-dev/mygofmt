// Package layoutstyle implements configuration-driven source layout.
package layoutstyle

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jiho-dev/mygofmt/internal/config"
)

// Apply changes source layout without changing its AST.
func Apply(source []byte, cfg config.Config) ([]byte, error) {
	if cfg.Rules.SpaceConfiguredCalls {
		var err error
		source, err = joinCallComments(source, cfg)
		if err != nil {
			return nil, err
		}
	}

	if !cfg.Rules.PreserveExistingBlankLines {
		var err error
		source, err = compactWhitespace(source)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Rules.JoinCallErrorChecks {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
		if err != nil {
			return nil, err
		}

		source = joinErrorChecks(source, fset, file)
	}

	var err error
	if cfg.Rules.CompactSingleStatementBlocks {
		source, err = compactSingleStatementBlocks(source)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Rules.BalanceLogicalConditions {
		source, err = balanceConditions(source, cfg)
		if err != nil {
			return nil, err
		}
	}

	for round := 0; round < 64; round++ {
		state, err := parseSource(source, cfg)
		if err != nil {
			return nil, err
		}

		state.wrapLines()
		state.expandBlocks()
		wrapped, err := format.Source(state.apply())
		if err != nil {
			return nil, fmt.Errorf("format layout: %w", err)
		}

		state, err = parseSource(wrapped, cfg)
		if err != nil {
			return nil, err
		}

		state.spaceBlocks()
		output, err := format.Source(state.apply())
		if err != nil {
			return nil, fmt.Errorf("format spacing: %w", err)
		}

		if bytes.Equal(source, output) {
			return output, nil
		}

		source = output
	}

	return nil, fmt.Errorf("layout formatting did not converge after 64 rounds")
}

type sourceState struct {
	source  []byte
	fset    *token.FileSet
	file    *ast.File
	options config.Config
	inserts map[int]string
	logs    map[string]config.CallRule
}

func parseSource(source []byte, options config.Config) (*sourceState, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	logs := make(map[string]config.CallRule)

	for _, rule := range options.Calls {
		for _, name := range rule.Methods {
			logs[name] = rule
		}
	}

	return &sourceState{source, fset, file, options, make(map[int]string), logs}, nil
}

func (s *sourceState) offset(pos token.Pos) int { return s.fset.PositionFor(pos, false).Offset }
func (s *sourceState) line(pos token.Pos) int   { return s.fset.PositionFor(pos, false).Line }

func (s *sourceState) insert(pos token.Pos, text string) {
	offset := s.offset(pos)
	old := s.inserts[offset]
	if old == "" || (strings.Contains(text, ",") && !strings.Contains(old, ",")) {
		s.inserts[offset] = text
	}
}

func (s *sourceState) apply() []byte {
	positions := make([]int, 0, len(s.inserts))

	for pos := range s.inserts {
		positions = append(positions, pos)
	}

	sort.Ints(positions)
	var out bytes.Buffer
	start := 0

	for _, pos := range positions {
		out.Write(s.source[start:pos])
		out.WriteString(s.inserts[pos])
		start = pos
	}

	out.Write(s.source[start:])

	return out.Bytes()
}

func (s *sourceState) longLine(pos token.Pos) bool {
	offset := s.offset(pos)
	start := bytes.LastIndexByte(s.source[:offset], '\n') + 1
	end := bytes.IndexByte(s.source[offset:], '\n')
	if end < 0 {
		end = len(s.source) - offset
	}

	line := s.source[start : offset+end]

	return s.lineWidth(line) > s.options.Layout.MaxLineLength
}

func (s *sourceState) lineWidth(line []byte) int {
	return utf8.RuneCount(line) + bytes.Count(line, []byte{'\t'})*(s.options.Layout.TabWidth-1)
}

// Measure through the closing brace rather than the rest of its physical line.
// A short receiver literal must not wrap because a following call is long.
func (s *sourceState) longLiteral(literal *ast.CompositeLit) bool {
	start := bytes.LastIndexByte(s.source[:s.offset(literal.Pos())], '\n') + 1

	for _, line := range bytes.Split(s.source[start:s.offset(literal.End())], []byte{'\n'}) {
		if s.lineWidth(line) > s.options.Layout.MaxLineLength {
			return true
		}
	}

	return false
}

// splitList preserves existing element text and comments. The separator comma
// precedes a trailing line comment, so the comment stays on its original item.
func (s *sourceState) splitList(open, close token.Pos, items []ast.Node, lastEnd token.Pos, force bool) {
	if len(items) == 0 {
		return
	}

	long := force || s.longLine(open) || s.longLine(close)

	for _, item := range items {
		long = long || s.longLine(item.Pos()) || s.longLine(item.End())
	}

	if !long {
		return
	}

	if s.line(open) == s.line(items[0].Pos()) {
		s.insert(open+1, "\n")
	}

	for i, item := range items {
		end := item.End()
		if i == len(items)-1 {
			end = lastEnd
		}

		next := close
		if i+1 < len(items) {
			next = items[i+1].Pos()
		}

		if s.line(end) != s.line(next) {
			continue
		}

		between := s.source[s.offset(end):s.offset(next)]
		// A block comment can occur between the element and its comma.
		separatorFile := token.NewFileSet().AddFile("", -1, len(between))
		var scan scanner.Scanner
		scan.Init(separatorFile, between, nil, 0)
		pos, tok, _ := scan.Scan()
		if tok == token.COMMA {
			comma := end + token.Pos(separatorFile.Offset(pos))
			s.insert(comma+1, "\n")
		} else {
			s.insert(end, ",\n")
		}
	}
}

func (s *sourceState) wrapLines() {
	nested := s.nestedLiterals()
	ast.Inspect(s.file, func(node ast.Node) bool {
		before := len(s.inserts)
		switch n := node.(type) {
		case *ast.CallExpr:
			if !s.options.Rules.WrapCallArguments || len(n.Args) < s.options.Layout.MinCallArguments {
				return true
			}

			items := make([]ast.Node, len(n.Args))

			for i, arg := range n.Args {
				items[i] = arg
			}

			if len(items) > 0 {
				end := items[len(items)-1].End()
				if n.Ellipsis.IsValid() {
					end = n.Ellipsis + 3
				}

				s.splitList(n.Lparen, n.Rparen, items, end, false)
			}
		case *ast.CompositeLit:
			if !s.options.Rules.WrapCompositeLiterals {
				return true
			}

			items := make([]ast.Node, len(n.Elts))

			for i, elt := range n.Elts {
				items[i] = elt
			}

			if len(items) > 0 {
				force := nested[n] || (keyedLiteral(n) && s.line(n.Lbrace) != s.line(n.Rbrace))
				if force || s.longLiteral(n) {
					s.splitList(n.Lbrace, n.Rbrace, items, items[len(items)-1].End(), true)
				}
			}
		case *ast.TypeSpec:
			if fields := n.TypeParams; s.options.Rules.WrapTypeParameters && fields != nil && len(fields.List) > 0 {
				items := make([]ast.Node, len(fields.List))

				for i, field := range fields.List {
					items[i] = field
				}

				s.splitList(fields.Opening, fields.Closing, items, items[len(items)-1].End(), false)
			}
		case *ast.FuncType:
			for i, fields := range []*ast.FieldList{n.TypeParams, n.Params, n.Results} {
				enabled := []bool{
					s.options.Rules.WrapTypeParameters,
					s.options.Rules.WrapFunctionParameters,
					s.options.Rules.WrapFunctionResults,
				}

				if !enabled[i] {
					continue
				}

				if fields == nil || !fields.Opening.IsValid() {
					continue
				}

				items := make([]ast.Node, len(fields.List))

				for i, field := range fields.List {
					items[i] = field
				}

				if len(items) > 0 {
					before := len(s.inserts)
					s.splitList(fields.Opening, fields.Closing, items, items[len(items)-1].End(), false)
					// Re-evaluate result width after parameters have been wrapped.
					if len(s.inserts) > before {
						break
					}
				}
			}
		}

		// Reparse after splitting an enclosing expression before measuring its
		// children. Short nested calls may fit once the outer line is split.
		return len(s.inserts) == before
	})
}

func keyedLiteral(literal *ast.CompositeLit) bool {
	if len(literal.Elts) == 0 {
		return false
	}

	_, keyed := literal.Elts[0].(*ast.KeyValueExpr)

	return keyed
}

// nestedLiterals carries multiline layout into direct keyed literal values.
// Short positional values such as []string{"host-a"} remain compact.
func (s *sourceState) nestedLiterals() map[*ast.CompositeLit]bool {
	nested := make(map[*ast.CompositeLit]bool)
	if !s.options.Rules.WrapNestedCompositeLiterals {
		return nested
	}

	ast.Inspect(s.file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok || s.line(literal.Lbrace) == s.line(literal.Rbrace) {
			return true
		}

		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			value := field.Value

			for {
				switch n := value.(type) {
				case *ast.UnaryExpr:
					if n.Op == token.AND {
						value = n.X
						continue
					}
				case *ast.ParenExpr:
					value = n.X
					continue
				}

				break
			}

			if child, ok := value.(*ast.CompositeLit); ok && keyedLiteral(child) {
				nested[child] = true
			}
		}

		return true
	})

	return nested
}

func (s *sourceState) callRule(stmt ast.Stmt) (config.CallRule, bool) {
	var expression ast.Expr
	switch n := stmt.(type) {
	case *ast.ExprStmt:
		expression = n.X
	case *ast.DeferStmt:
		expression = n.Call
	case *ast.GoStmt:
		expression = n.Call
	default:
		return config.CallRule{}, false
	}

	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return config.CallRule{}, false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return config.CallRule{}, false
	}

	rule, matched := s.logs[selector.Sel.Name]
	if matched && len(rule.Receivers) > 0 {
		matched = slices.Contains(rule.Receivers, receiverName(selector.X))
	}

	return rule, matched
}

// receiverName matches configured source names without guessing receiver types.
func receiverName(expression ast.Expr) string {
	switch n := expression.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		if prefix := receiverName(n.X); prefix != "" {
			return prefix + "." + n.Sel.Name
		}
	case *ast.ParenExpr:
		return receiverName(n.X)
	}

	return ""
}

func (s *sourceState) callSpacing(stmt ast.Stmt, before, boundary bool) bool {
	if !s.options.Rules.SpaceConfiguredCalls {
		return false
	}

	rule, ok := s.callRule(stmt)
	if !ok || (boundary && !rule.IncludeBlockBoundaries) {
		return false
	}

	if before {
		return rule.BlankBefore
	}

	return rule.BlankAfter
}

func (s *sourceState) expandBlocks() {
	deferred := make(map[*ast.BlockStmt]bool)
	ast.Inspect(s.file, func(node ast.Node) bool {
		if n, ok := node.(*ast.DeferStmt); ok && s.options.Rules.MultilineDeferBody {
			if literal, ok := n.Call.Fun.(*ast.FuncLit); ok {
				deferred[literal.Body] = true
			}
		}

		return true
	})
	ast.Inspect(s.file, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}

		if len(block.List) == 0 {
			if deferred[block] && s.line(block.Lbrace) == s.line(block.Rbrace) {
				s.insert(block.Lbrace+1, "\n")
				s.insert(block.Rbrace, "\n")
			}

			return true
		}

		expand := deferred[block]
		if len(block.List) > 1 {
			expand = expand || s.callSpacing(block.List[0], true, true) ||
				s.callSpacing(block.List[len(block.List)-1], false, true)
		}

		for i := 1; i < len(block.List); i++ {
			expand = expand || s.needsGap(block.List[i-1], block.List[i])
		}

		if !expand {
			return true
		}

		if s.line(block.Lbrace) == s.line(block.List[0].Pos()) {
			s.insert(block.Lbrace+1, "\n")
		}

		for i := 1; i < len(block.List); i++ {
			if s.line(block.List[i-1].End()) == s.line(block.List[i].Pos()) {
				s.insert(block.List[i].Pos(), "\n")
			}
		}

		if s.line(block.List[len(block.List)-1].End()) == s.line(block.Rbrace) {
			s.insert(block.Rbrace, "\n")
		}

		return true
	})
}

func isLoop(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return true
	}

	return false
}

func isControl(stmt ast.Stmt) bool {
	switch stmt.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}

	return false
}

// commentStart attaches adjacent full-line comments to the following statement,
// but leaves same-line trailing comments with the previous statement.
func (s *sourceState) commentStart(after, before token.Pos) token.Pos {
	start := before

	for i := len(s.file.Comments) - 1; i >= 0; i-- {
		group := s.file.Comments[i]
		if group.Pos() <= after || group.End() > start {
			continue
		}

		if s.line(group.Pos()) == s.line(after) {
			continue
		}

		if s.line(group.End())+1 < s.line(start) {
			break
		}

		start = group.Pos()
	}

	return start
}

func (s *sourceState) blankBefore(after, before token.Pos) {
	start := s.commentStart(after, before)
	if s.line(after) >= s.line(start) {
		return
	}

	// Comments left with the preceding statement are not whitespace gaps.
	for _, group := range s.file.Comments {
		for _, comment := range group.List {
			if comment.Pos() >= after && comment.End() <= start {
				after = comment.End()
			}
		}
	}

	if s.line(after) == s.line(start) {
		// A trailing block comment can close on the next statement's line.
		s.insert(start, "\n\n")

		return
	}

	gap := s.source[s.offset(after):s.offset(start)]
	// Inspect actual empty lines: a multiline comment is not an empty line.
	lines := bytes.Split(gap, []byte{'\n'})

	for _, line := range lines[1 : len(lines)-1] {
		if len(bytes.TrimSpace(line)) == 0 {
			return
		}
	}

	offset := s.offset(start)
	lineStart := bytes.LastIndexByte(s.source[:offset], '\n') + 1
	s.inserts[lineStart] = "\n"
}

func (s *sourceState) spaceStatements(statements []ast.Stmt, open, close token.Pos) {
	if len(statements) <= 1 {
		return
	}

	if s.callSpacing(statements[0], true, true) {
		s.blankBefore(open, statements[0].Pos())
	}

	for i := 1; i < len(statements); i++ {
		previous, current := statements[i-1], statements[i]
		if s.needsGap(previous, current) {
			s.blankBefore(previous.End(), current.Pos())
		}
	}

	if s.callSpacing(statements[len(statements)-1], false, true) {
		s.blankBefore(statements[len(statements)-1].End(), close)
	}
}

func (s *sourceState) needsGap(previous, current ast.Stmt) bool {
	_, returning := current.(*ast.ReturnStmt)
	w := s.options.Rules

	return s.callSpacing(previous, false, false) || s.callSpacing(current, true, false) || (w.BlankAfterLoop &&
		isLoop(previous)) || (w.BlankBeforeLoop && isLoop(current)) || (w.BlankAfterControlBlock &&
		isControl(previous) && !isLoop(previous)) || (w.BlankBeforeReturn && returning) || (w.BlankAfterMultilineInit &&
		s.longInitialization(previous))
}

func (s *sourceState) longInitialization(stmt ast.Stmt) bool {
	assignment, ok := stmt.(*ast.AssignStmt)
	if !ok {
		return false
	}

	for _, value := range assignment.Rhs {
		if literal, ok := value.(*ast.CompositeLit); ok && s.line(literal.Pos()) != s.line(literal.End()) {
			return true
		}
	}

	return false
}

func (s *sourceState) spaceBlocks() {
	ast.Inspect(s.file, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return true
		}

		s.spaceStatements(block.List, block.Lbrace, block.Rbrace)

		// Clause boundaries are available directly in their containing block.
		for i, stmt := range block.List {
			end := block.Rbrace
			if i+1 < len(block.List) {
				end = block.List[i+1].Pos()
			}

			switch n := stmt.(type) {
			case *ast.CaseClause:
				s.spaceStatements(n.Body, n.Colon, end)
			case *ast.CommClause:
				s.spaceStatements(n.Body, n.Colon, end)
			}
		}

		return true
	})
}

// joinErrorChecks only joins a call assignment and a plain err != nil guard.
// A comment or a different condition leaves the author's spacing untouched.
func joinErrorChecks(source []byte, fset *token.FileSet, file *ast.File) []byte {
	type gap struct{ start, end int }
	var gaps []gap
	ast.Inspect(file, func(node ast.Node) bool {
		var statements []ast.Stmt
		switch n := node.(type) {
		case *ast.BlockStmt:
			statements = n.List
		case *ast.CaseClause:
			statements = n.Body
		case *ast.CommClause:
			statements = n.Body
		default:
			return true
		}

		for i := 1; i < len(statements); i++ {
			assignment, ok := statements[i-1].(*ast.AssignStmt)
			if !ok || len(assignment.Rhs) != 1 {
				continue
			}

			if _, ok := assignment.Rhs[0].(*ast.CallExpr); !ok {
				continue
			}

			guard, ok := statements[i].(*ast.IfStmt)
			if !ok || guard.Init != nil {
				continue
			}

			condition, ok := guard.Cond.(*ast.BinaryExpr)
			if !ok || condition.Op != token.NEQ {
				continue
			}

			identifier, ok := condition.X.(*ast.Ident)
			if !ok || (identifier.Name != "err" && !strings.HasSuffix(identifier.Name, "Err")) {
				continue
			}

			nilValue, ok := condition.Y.(*ast.Ident)
			if !ok || nilValue.Name != "nil" {
				continue
			}

			assigned := false

			for _, lhs := range assignment.Lhs {
				if name, ok := lhs.(*ast.Ident); ok && name.Name == identifier.Name {
					assigned = true
				}
			}

			if !assigned {
				continue
			}

			start := fset.Position(assignment.End()).Offset
			end := fset.Position(guard.Pos()).Offset
			if len(bytes.TrimSpace(source[start:end])) == 0 && bytes.Count(source[start:end], []byte{'\n'}) > 1 {
				gaps = append(gaps, gap{start, end})
			}
		}

		return true
	})
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].start < gaps[j].start })
	var output bytes.Buffer
	start := 0

	for _, gap := range gaps {
		output.Write(source[start:gap.start])
		output.WriteByte('\n')
		start = gap.end
	}

	output.Write(source[start:])

	return output.Bytes()
}
