package formatter

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"slices"
	"sort"

	"github.com/jiho-dev/mygofmt/internal/config"
)

type signatureFields [3]*ast.FieldList

type signatureEdit struct {
	start, end int
	text       []byte
}

// Gofumpt can add a trailing comma and move a function signature's closing
// delimiter even when mygofmt's signature wrapping options are disabled.
// Restore only lists whose tokens are otherwise unchanged by gofumpt.
func restoreSignatureLayout(original, formatted []byte, wrap config.Wrap) ([]byte, error) {
	if wrap.TypeParameters && wrap.FunctionParameters && wrap.FunctionResults {
		return formatted, nil
	}
	originalSet := token.NewFileSet()
	originalFile, err := parser.ParseFile(originalSet, "", original, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	formattedSet := token.NewFileSet()
	formattedFile, err := parser.ParseFile(formattedSet, "", formatted, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	originalLists := functionSignatures(originalFile)
	formattedLists := functionSignatures(formattedFile)
	if len(originalLists) != len(formattedLists) {
		return formatted, nil
	}
	enabled := [3]bool{wrap.TypeParameters, wrap.FunctionParameters, wrap.FunctionResults}
	var edits []signatureEdit
	for i, lists := range originalLists {
		for j, fields := range lists {
			other := formattedLists[i][j]
			if enabled[j] || fields == nil || other == nil || !fields.Opening.IsValid() || !other.Opening.IsValid() {
				continue
			}
			start := originalSet.PositionFor(fields.Opening, false).Offset
			end := originalSet.PositionFor(fields.Closing, false).Offset + 1
			otherStart := formattedSet.PositionFor(other.Opening, false).Offset
			otherEnd := formattedSet.PositionFor(other.Closing, false).Offset + 1
			oldText := original[start:end]
			newText := formatted[otherStart:otherEnd]
			if !slices.Equal(signatureTokens(oldText), signatureTokens(newText)) || bytes.Equal(oldText, newText) {
				continue
			}
			edits = append(edits, signatureEdit{otherStart, otherEnd, oldText})
		}
	}
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start == edits[j].start {
			return edits[i].end > edits[j].end
		}
		return edits[i].start < edits[j].start
	})
	var result bytes.Buffer
	last := 0
	for _, edit := range edits {
		if edit.start < last { // An enclosing signature already restores this list.
			continue
		}
		result.Write(formatted[last:edit.start])
		result.Write(edit.text)
		last = edit.end
	}
	result.Write(formatted[last:])
	return result.Bytes(), nil
}

func functionSignatures(file *ast.File) []signatureFields {
	var lists []signatureFields
	ast.Inspect(file, func(node ast.Node) bool {
		if fn, ok := node.(*ast.FuncType); ok {
			lists = append(lists, signatureFields{fn.TypeParams, fn.Params, fn.Results})
		}
		return true
	})
	return lists
}

type signatureToken struct {
	kind token.Token
	lit  string
}

func signatureTokens(source []byte) []signatureToken {
	file := token.NewFileSet().AddFile("", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, source, nil, scanner.ScanComments)
	var tokens []signatureToken
	for {
		_, kind, lit := lexer.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.SEMICOLON && lit == "\n" {
			continue
		}
		tokens = append(tokens, signatureToken{kind, lit})
	}
	// Gofumpt adds a trailing comma when moving a closing delimiter to its
	// own line. That punctuation is the only token change allowed here.
	if len(tokens) >= 2 && tokens[len(tokens)-2].kind == token.COMMA {
		tokens = append(tokens[:len(tokens)-2], tokens[len(tokens)-1])
	}
	return tokens
}
