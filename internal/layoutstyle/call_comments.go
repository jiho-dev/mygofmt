package layoutstyle

import (
	"bytes"
	"go/ast"
	"sort"

	"github.com/jiho-dev/mygofmt/internal/config"
)

// joinCallComments keeps the nearest standalone comment group with its call.
// Only whitespace between the group and the call is removed; earlier comment
// paragraphs and comments trailing other code remain untouched.
func joinCallComments(source []byte, cfg config.Config) ([]byte, error) {
	s, err := parseSource(source, cfg)
	if err != nil {
		return nil, err
	}

	type gap struct{ start, end int }
	var gaps []gap
	ast.Inspect(s.file, func(node ast.Node) bool {
		stmt, ok := node.(ast.Stmt)
		if !ok {
			return true
		}
		if _, matched := s.callRule(stmt); !matched {
			return true
		}

		i := sort.Search(len(s.file.Comments), func(i int) bool {
			return s.file.Comments[i].End() > stmt.Pos()
		}) - 1
		if i < 0 {
			return true
		}

		group := s.file.Comments[i]
		commentStart := s.offset(group.Pos())
		lineStart := bytes.LastIndexByte(source[:commentStart], '\n') + 1
		if len(bytes.TrimSpace(source[lineStart:commentStart])) != 0 {
			return true
		}

		start, end := s.offset(group.End()), s.offset(stmt.Pos())
		between := source[start:end]
		if len(bytes.TrimSpace(between)) == 0 && bytes.Count(between, []byte{'\n'}) > 1 {
			gaps = append(gaps, gap{start, end})
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
