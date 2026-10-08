package layoutstyle

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"sort"

	"github.com/jiho-dev/mygofmt/internal/config"
)

// balanceConditions normalizes the whitespace following logical operators.
// Expressions containing comments or multiline operands keep their layout.
func balanceConditions(source []byte, cfg config.Config) ([]byte, error) {
	s, err := parseSource(source, cfg)
	if err != nil {
		return nil, err
	}
	type edit struct {
		start, end int
		text       []byte
	}
	var edits []edit
	ast.Inspect(s.file, func(node ast.Node) bool {
		root, ok := node.(*ast.BinaryExpr)
		if !ok || !logical(root) {
			return true
		}
		for _, comment := range s.file.Comments {
			if comment.Pos() < root.End() && comment.End() > root.Pos() {
				return false
			}
		}
		var operators []*ast.BinaryExpr
		var collect func(ast.Expr)
		collect = func(expr ast.Expr) {
			switch n := expr.(type) {
			case *ast.BinaryExpr:
				if logical(n) {
					collect(n.X)
					operators = append(operators, n)
					collect(n.Y)
				}
			case *ast.ParenExpr:
				collect(n.X)
			}
		}
		collect(root)
		sort.Slice(operators, func(i, j int) bool { return operators[i].OpPos < operators[j].OpPos })
		start, end := s.offset(root.Pos()), s.offset(root.End())
		var units [][]byte
		cursor := start
		for _, operator := range operators {
			opEnd := s.offset(operator.OpPos) + len(operator.Op.String())
			next := s.offset(operator.Y.Pos())
			if len(bytes.TrimSpace(source[opEnd:next])) != 0 {
				return false
			}
			units = append(units, source[cursor:opEnd])
			cursor = next
		}
		units = append(units, source[cursor:end])
		for _, unit := range units {
			if bytes.ContainsAny(unit, "\r\n") {
				return false
			}
		}
		lineStart := bytes.LastIndexByte(source[:start], '\n') + 1
		lineEnd := bytes.IndexByte(source[end:], '\n')
		if lineEnd < 0 {
			lineEnd = len(source) - end
		}
		indentEnd := lineStart
		for indentEnd < start && (source[indentEnd] == ' ' || source[indentEnd] == '\t') {
			indentEnd++
		}
		prefix := s.lineWidth(source[lineStart:start])
		continuation := s.lineWidth(source[lineStart:indentEnd]) + cfg.Layout.TabWidth
		suffix := s.lineWidth(source[end : end+lineEnd])
		text := s.balancedLogicalLines(units, prefix, continuation, suffix)
		if !bytes.Equal(source[start:end], text) {
			edits = append(edits, edit{start, end, text})
		}
		return false
	})
	if len(edits) == 0 {
		return source, nil
	}
	var output bytes.Buffer
	cursor := 0
	for _, edit := range edits {
		output.Write(source[cursor:edit.start])
		output.Write(edit.text)
		cursor = edit.end
	}
	output.Write(source[cursor:])
	return format.Source(output.Bytes())
}

func logical(expr *ast.BinaryExpr) bool {
	return expr.Op == token.LAND || expr.Op == token.LOR
}

func (s *sourceState) balancedLogicalLines(units [][]byte, prefix, continuation, suffix int) []byte {
	widths := make([]int, len(units)+1)
	for i, unit := range units {
		widths[i+1] = widths[i] + s.lineWidth(unit)
	}
	width := func(start, end int) int {
		result := widths[end] - widths[start] + end - start - 1
		if start == 0 {
			result += prefix
		} else {
			result += continuation
		}
		if end == len(units) {
			result += suffix
		}
		return result
	}
	limit := s.options.Layout.MaxLineLength
	if width(0, len(units)) <= limit {
		return bytes.Join(units, []byte(" "))
	}
	best, difference := -1, int(^uint(0)>>1)
	for split := 1; split < len(units); split++ {
		first, second := width(0, split), width(split, len(units))
		if first > limit || second > limit {
			continue
		}
		delta := first - second
		if delta < 0 {
			delta = -delta
		}
		if delta < difference {
			best, difference = split, delta
		}
	}
	if best >= 0 {
		return bytes.Join([][]byte{bytes.Join(units[:best], []byte(" ")), bytes.Join(units[best:], []byte(" "))}, []byte("\n"))
	}
	// If two lines cannot fit, fill each line without splitting an operand.
	var lines [][]byte
	for start := 0; start < len(units); {
		end := start + 1
		for end < len(units) && width(start, end+1) <= limit {
			end++
		}
		lines = append(lines, bytes.Join(units[start:end], []byte(" ")))
		start = end
	}
	return bytes.Join(lines, []byte("\n"))
}
