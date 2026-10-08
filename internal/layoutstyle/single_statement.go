package layoutstyle

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// A block with one statement needs no empty line against either brace.
// Leave comments and authored statement contents untouched.
func compactSingleStatementBlocks(source []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	type gap struct{ start, end int }
	var gaps []gap
	ast.Inspect(file, func(node ast.Node) bool {
		block, ok := node.(*ast.BlockStmt)
		if !ok || len(block.List) != 1 {
			return true
		}
		stmt := block.List[0]
		for _, positions := range [][2]token.Pos{{block.Lbrace + 1, stmt.Pos()}, {stmt.End(), block.Rbrace}} {
			start := fset.PositionFor(positions[0], false).Offset
			end := fset.PositionFor(positions[1], false).Offset
			between := source[start:end]
			if len(bytes.TrimSpace(between)) == 0 && bytes.Count(between, []byte{'\n'}) > 1 {
				gaps = append(gaps, gap{start, end})
			}
		}
		return true
	})
	if len(gaps) == 0 {
		return source, nil
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].start < gaps[j].start })
	var output bytes.Buffer
	start := 0
	for _, gap := range gaps {
		output.Write(source[start:gap.start])
		output.WriteByte('\n')
		start = gap.end
	}
	output.Write(source[start:])
	return output.Bytes(), nil
}
