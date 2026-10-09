package diff

import (
	"errors"
	"strings"
	"unicode/utf8"

	"ppgit-go/internal/core"
)

const MaxMergeRows = 10000

func Merge(c *core.Core, projectID, from, to, path, expectedFrom, expectedTo string, selected []int) (map[string]any, error) {
	if len(selected) == 0 || len(selected) > MaxMergeRows {
		return nil, errors.New("select between 1 and 10000 changed rows")
	}
	result, err := c.UpdateVersionFile(projectID, from, to, path, expectedFrom, expectedTo, func(old, current []byte) ([]byte, error) {
		return mergeRows(path, string(old), string(current), selected)
	})
	if err == nil {
		result["applied_rows"] = len(selected)
	}
	return result, err
}

type rawLine struct{ text, ending string }

func rawLines(text string) []rawLine {
	result := []rawLine{}
	start := 0
	for index := 0; index < len(text); {
		r, width := utf8.DecodeRuneInString(text[index:])
		end := index + width
		switch r {
		case '\r':
			if end < len(text) && text[end] == '\n' {
				end++
			}
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
		default:
			index = end
			continue
		}
		result = append(result, rawLine{text[start:index], text[index:end]})
		start, index = end, end
	}
	if start < len(text) {
		result = append(result, rawLine{text[start:], ""})
	}
	return result
}

func mergeRows(path, old, current string, selected []int) ([]byte, error) {
	preview, err := BuildPreview(path, old, current, true, false, nil)
	if err != nil {
		return nil, err
	}
	picks := make(map[int]bool, len(selected))
	for _, index := range selected {
		if index < 0 || index >= len(preview.Rows) || preview.Rows[index].Status == "same" || picks[index] {
			return nil, errors.New("selection must contain unique, valid changed row indices")
		}
		picks[index] = true
	}
	left, right := rawLines(old), rawLines(current)
	lines := make([]rawLine, 0, len(preview.Rows))
	for index, row := range preview.Rows {
		if picks[index] {
			if row.LeftNo != nil {
				lines = append(lines, left[*row.LeftNo-1])
			}
		} else if row.RightNo != nil {
			lines = append(lines, right[*row.RightNo-1])
		}
	}
	separator := "\n"
	for _, source := range [][]rawLine{left, right} {
		for _, line := range source {
			if line.ending != "" {
				separator = line.ending
				break
			}
		}
	}
	var output strings.Builder
	for index, line := range lines {
		output.WriteString(line.text)
		output.WriteString(line.ending)
		if line.ending == "" && index != len(lines)-1 {
			output.WriteString(separator)
		}
	}
	return []byte(output.String()), nil
}

func mergeAvailability(from, to map[string]any, left, right core.Entry, hasLeft, hasRight bool) (string, string) {
	if sequence(from["sequence"]) >= sequence(to["sequence"]) {
		return "invalid_order", "\u8bf7\u9009\u62e9\u4e0d\u540c\u7684\u65e7\u7248\u672c\u548c\u65b0\u7248\u672c"
	}
	if reason, _ := to["read_only_reason"].(string); reason != "" {
		return "version_read_only", reason
	}
	if (hasLeft && !left.IsText) || (hasRight && !right.IsText) {
		return "binary_file", "\u4ec5\u652f\u6301\u5408\u5e76 UTF-8 \u6587\u672c\u6587\u4ef6"
	}
	return "", ""
}
