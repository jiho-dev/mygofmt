package formatter

import (
	"bytes"
	"go/scanner"
	"go/token"
	"sort"
)

type blockComment struct {
	text             string
	start, end       int
	gapStart, gapEnd int
}

// Gofumpt can remove authored empty lines next to block comments, or attach a
// closing marker to the next statement. Preserve those gaps around matching comments.
func restoreBlockCommentGaps(original, formatted []byte) []byte {
	oldComments := blockComments(original)
	newComments := blockComments(formatted)
	if len(oldComments) != len(newComments) {
		return formatted
	}

	type edit struct {
		start, end int
		text       []byte
	}
	var edits []edit

	for i, old := range oldComments {
		newComment := newComments[i]
		if old.text != newComment.text {
			continue
		}

		oldBefore := original[old.gapStart:old.start]
		newBefore := formatted[newComment.gapStart:newComment.start]
		if bytes.Count(oldBefore, []byte{'\n'}) > bytes.Count(newBefore, []byte{'\n'}) {
			edits = append(edits, edit{newComment.gapStart, newComment.start, oldBefore})
		}

		oldGap := original[old.end:old.gapEnd]
		newGap := formatted[newComment.end:newComment.gapEnd]
		if bytes.Count(oldGap, []byte{'\n'}) > bytes.Count(newGap, []byte{'\n'}) {
			edits = append(edits, edit{newComment.end, newComment.gapEnd, oldGap})
		}
	}

	if len(edits) == 0 {
		return formatted
	}

	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var output bytes.Buffer
	last := 0

	for _, edit := range edits {
		if edit.start < last {
			continue
		}

		output.Write(formatted[last:edit.start])
		output.Write(edit.text)
		last = edit.end
	}

	output.Write(formatted[last:])

	return output.Bytes()
}

func blockComments(source []byte) []blockComment {
	file := token.NewFileSet().AddFile("", -1, len(source))
	var lexer scanner.Scanner
	lexer.Init(file, source, nil, scanner.ScanComments)
	var comments []blockComment

	for {
		pos, kind, lit := lexer.Scan()
		if kind == token.EOF {
			break
		}

		if kind != token.COMMENT || len(lit) < 2 || lit[:2] != "/*" {
			continue
		}

		start := file.Offset(pos)
		end := start + len(lit)
		gapStart := start

		for gapStart > 0 && (source[gapStart-1] == ' ' || source[gapStart-1] == '\t' ||
			source[gapStart-1] == '\r' || source[gapStart-1] == '\n') {
			gapStart--
		}

		gapEnd := end

		for gapEnd < len(source) && (source[gapEnd] == ' ' || source[gapEnd] == '\t' ||
			source[gapEnd] == '\r' || source[gapEnd] == '\n') {
			gapEnd++
		}

		comments = append(comments, blockComment{lit, start, end, gapStart, gapEnd})
	}

	return comments
}
