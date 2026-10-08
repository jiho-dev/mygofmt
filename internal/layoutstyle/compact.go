package layoutstyle

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// compactWhitespace edits only whitespace-only gaps between AST statements.
// Multiline literals and comment text can never be part of one of these gaps.
func compactWhitespace(source []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	type gap struct{ start, end int }
	gaps := make(map[int]int)
	add := func(start, end token.Pos) {
		a, b := fset.PositionFor(start, false).Offset, fset.PositionFor(end, false).Offset
		if a < b && len(bytes.TrimSpace(source[a:b])) == 0 && bytes.Count(source[a:b], []byte{'\n'}) > 1 {
			gaps[a] = b
		}
	}
	statements := func(list []ast.Stmt, start, end token.Pos) {
		for _, stmt := range list {
			add(start, stmt.Pos())
			start = stmt.End()
		}
		add(start, end)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.BlockStmt:
			statements(n.List, n.Lbrace+1, n.Rbrace)
		case *ast.CaseClause:
			statements(n.Body, n.Colon+1, n.End())
		case *ast.CommClause:
			statements(n.Body, n.Colon+1, n.End())
		}
		return true
	})
	ordered := make([]gap, 0, len(gaps))
	for start, end := range gaps {
		ordered = append(ordered, gap{start, end})
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].start < ordered[j].start })
	var output bytes.Buffer
	start := 0
	for _, gap := range ordered {
		output.Write(source[start:gap.start])
		output.WriteByte('\n')
		start = gap.end
	}
	output.Write(source[start:])
	return output.Bytes(), nil
}
