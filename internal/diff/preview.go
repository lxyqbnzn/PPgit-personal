package diff

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

const (
	MaxInputBytes   = 4 * 1024 * 1024
	MaxOutputBytes  = 4 * 1024 * 1024
	MaxLines        = 40_000
	MaxMatchProduct = 4_000_000
	MaxMatchPairs   = 200_000
	TimeBudget      = time.Second
	LimitMessage    = "[\u5dee\u5f02\u5185\u5bb9\u8d85\u8fc7\u5b89\u5168\u9884\u89c8\u9884\u7b97\uff0c\u8bf7\u6309\u6587\u4ef6\u5bf9\u6bd4\u6216\u4f7f\u7528\u5916\u90e8\u5bf9\u6bd4\u5de5\u5177]"
)

var ErrBudgetExceeded = errors.New("diff preview budget exceeded")

type Budget struct {
	InputRemaining  int64
	OutputRemaining int64
	Deadline        time.Time
}

func NewBudget() *Budget {
	return &Budget{MaxInputBytes, MaxOutputBytes, time.Now().Add(TimeBudget)}
}

func (b *Budget) Check() error {
	if !b.Deadline.IsZero() && time.Now().After(b.Deadline) {
		return ErrBudgetExceeded
	}
	return nil
}

func (b *Budget) ReserveInput(size int64) error {
	if err := b.Check(); err != nil {
		return err
	}
	if size < 0 || size > b.InputRemaining {
		return ErrBudgetExceeded
	}
	b.InputRemaining -= size
	return nil
}

func (b *Budget) reserveOutput(size int) error {
	if err := b.Check(); err != nil {
		return err
	}
	if size < 0 || int64(size) > b.OutputRemaining {
		return ErrBudgetExceeded
	}
	b.OutputRemaining -= int64(size)
	return nil
}

type Row struct {
	Status    string `json:"status"`
	LeftNo    *int   `json:"left_no"`
	RightNo   *int   `json:"right_no"`
	LeftText  string `json:"left_text"`
	RightText string `json:"right_text"`
}

type Preview struct {
	Patch *string `json:"patch"`
	Rows  []Row   `json:"diff_rows"`
}

func BuildPreview(path, oldText, newText string, details, patch bool, budget *Budget) (Preview, error) {
	if budget == nil {
		budget = NewBudget()
	}
	if !details && !patch {
		return Preview{}, nil
	}
	if err := budget.ReserveInput(int64(len(oldText)) + int64(len(newText))); err != nil {
		return Preview{}, err
	}
	return buildPreviewReserved(path, oldText, newText, details, patch, budget)
}

func buildPreviewReserved(path, oldText, newText string, details, patch bool, budget *Budget) (Preview, error) {
	if len(oldText) > MaxInputBytes || len(newText) > MaxInputBytes-len(oldText) {
		return Preview{}, ErrBudgetExceeded
	}
	oldLines, err := splitLines(oldText, MaxLines)
	if err != nil {
		return Preview{}, err
	}
	newLines, err := splitLines(newText, MaxLines-len(oldLines))
	if err != nil {
		return Preview{}, err
	}
	opcodes, err := boundedOpCodes(oldLines, newLines, budget)
	if err != nil {
		return Preview{}, err
	}
	if err := budget.reserveOutput(64); err != nil {
		return Preview{}, err
	}
	preview := Preview{}
	if patch {
		text, err := buildPatch(path, oldLines, newLines, opcodes, budget)
		if err != nil {
			return Preview{}, err
		}
		if text != "" {
			preview.Patch = &text
		}
	}
	if details {
		rows, err := buildRows(oldLines, newLines, opcodes, budget)
		if err != nil {
			return Preview{}, err
		}
		preview.Rows = rows
	}
	if err := budget.Check(); err != nil {
		return Preview{}, err
	}
	return preview, nil
}

func splitLines(text string, limit int) ([]string, error) {
	lines := make([]string, 0)
	start, skipLF := 0, false
	for index, r := range text {
		if skipLF {
			skipLF = false
			if r == '\n' {
				start = index + 1
				continue
			}
		}
		width := 1
		switch r {
		case '\n', '\r', '\v', '\f', '\x1c', '\x1d', '\x1e':
		case '\u0085':
			width = 2
		case '\u2028', '\u2029':
			width = 3
		default:
			continue
		}
		if len(lines) >= limit {
			return nil, ErrBudgetExceeded
		}
		lines = append(lines, text[start:index])
		start = index + width
		skipLF = r == '\r'
	}
	if start < len(text) {
		if len(lines) >= limit {
			return nil, ErrBudgetExceeded
		}
		lines = append(lines, text[start:])
	}
	return lines, nil
}

func boundedOpCodes(old, current []string, budget *Budget) ([]difflib.OpCode, error) {
	if err := budget.Check(); err != nil {
		return nil, err
	}
	prefix := 0
	for prefix < len(old) && prefix < len(current) && old[prefix] == current[prefix] {
		prefix++
	}
	oldEnd, newEnd := len(old), len(current)
	for oldEnd > prefix && newEnd > prefix && old[oldEnd-1] == current[newEnd-1] {
		oldEnd--
		newEnd--
	}
	left, right := old[prefix:oldEnd], current[prefix:newEnd]
	counts := make(map[string]int, len(right))
	for _, line := range right {
		counts[line]++
	}
	pairs := int64(0)
	for _, line := range left {
		pairs += int64(counts[line])
		if pairs > MaxMatchPairs {
			return nil, ErrBudgetExceeded
		}
	}
	if pairs > 0 && int64(len(left))*int64(len(right)) > MaxMatchProduct {
		return nil, ErrBudgetExceeded
	}
	if err := budget.Check(); err != nil {
		return nil, err
	}
	matcher := difflib.NewMatcherWithJunk(left, right, false, nil)
	middle := matcher.GetOpCodes()
	opcodes := make([]difflib.OpCode, 0, len(middle)+2)
	if prefix > 0 {
		opcodes = append(opcodes, difflib.OpCode{Tag: 'e', I2: prefix, J2: prefix})
	}
	for _, code := range middle {
		code.I1 += prefix
		code.I2 += prefix
		code.J1 += prefix
		code.J2 += prefix
		opcodes = append(opcodes, code)
	}
	if oldEnd < len(old) {
		opcodes = append(opcodes, difflib.OpCode{Tag: 'e', I1: oldEnd, I2: len(old), J1: newEnd, J2: len(current)})
	}
	if err := budget.Check(); err != nil {
		return nil, err
	}
	return opcodes, nil
}

func buildRows(old, current []string, opcodes []difflib.OpCode, budget *Budget) ([]Row, error) {
	rows := make([]Row, 0)

	appendRow := func(status string, left, right int) error {
		row := Row{Status: status}
		if left >= 0 {
			number := left + 1
			row.LeftNo, row.LeftText = &number, old[left]
		}
		if right >= 0 {
			number := right + 1
			row.RightNo, row.RightText = &number, current[right]
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if err := budget.reserveOutput(len(encoded) + 1); err != nil {
			return err
		}
		rows = append(rows, row)
		return nil
	}
	for _, code := range opcodes {
		if code.Tag == 'e' {
			for offset := 0; offset < code.I2-code.I1; offset++ {
				if err := appendRow("same", code.I1+offset, code.J1+offset); err != nil {
					return nil, err
				}
			}
			continue
		}
		paired := 0
		if code.Tag == 'r' {
			paired = min(code.I2-code.I1, code.J2-code.J1)
		}
		for offset := 0; offset < paired; offset++ {
			if err := appendRow("modified", code.I1+offset, code.J1+offset); err != nil {
				return nil, err
			}
		}
		for left := code.I1 + paired; left < code.I2; left++ {
			if err := appendRow("deleted", left, -1); err != nil {
				return nil, err
			}
		}
		for right := code.J1 + paired; right < code.J2; right++ {
			if err := appendRow("added", -1, right); err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func groupedOpCodes(opcodes []difflib.OpCode, context int) [][]difflib.OpCode {
	var groups [][]difflib.OpCode
	var group []difflib.OpCode
	for index, original := range opcodes {
		code := original
		if code.Tag == 'e' {
			length := code.I2 - code.I1
			if index == 0 {
				code.I1, code.J1 = max(code.I1, code.I2-context), max(code.J1, code.J2-context)
			} else if index == len(opcodes)-1 {
				code.I2, code.J2 = min(code.I2, code.I1+context), min(code.J2, code.J1+context)
			} else if length > 2*context {
				group = append(group, difflib.OpCode{Tag: 'e', I1: code.I1, I2: code.I1 + context, J1: code.J1, J2: code.J1 + context})
				groups = append(groups, group)
				group = nil
				code.I1, code.J1 = code.I2-context, code.J2-context
			}
		}
		group = append(group, code)
	}
	if len(group) > 0 && !(len(group) == 1 && group[0].Tag == 'e') {
		groups = append(groups, group)
	}
	return groups
}

func unifiedRange(start, stop int) string {
	length := stop - start
	beginning := start + 1
	if length == 0 {
		beginning = start
	}
	if length == 1 {
		return fmt.Sprint(beginning)
	}
	return fmt.Sprintf("%d,%d", beginning, length)
}

func buildPatch(path string, old, current []string, opcodes []difflib.OpCode, budget *Budget) (string, error) {
	var output strings.Builder

	appendLine := func(line string) error {
		encoded, err := json.Marshal(line + "\n")
		if err != nil {
			return err
		}
		if err := budget.reserveOutput(len(encoded) - 2); err != nil {
			return err
		}
		output.WriteString(line)
		output.WriteByte('\n')
		return nil
	}
	for _, group := range groupedOpCodes(opcodes, 3) {
		if output.Len() == 0 {
			for _, line := range []string{"diff --git a/" + path + " b/" + path, "--- a/" + path, "+++ b/" + path} {
				if err := appendLine(line); err != nil {
					return "", err
				}
			}
		}
		first, last := group[0], group[len(group)-1]
		if err := appendLine(fmt.Sprintf("@@ -%s +%s @@", unifiedRange(first.I1, last.I2), unifiedRange(first.J1, last.J2))); err != nil {
			return "", err
		}
		for _, code := range group {
			if code.Tag == 'e' {
				for index := code.I1; index < code.I2; index++ {
					if err := appendLine(" " + old[index]); err != nil {
						return "", err
					}
				}
			}
			if code.Tag == 'r' || code.Tag == 'd' {
				for index := code.I1; index < code.I2; index++ {
					if err := appendLine("-" + old[index]); err != nil {
						return "", err
					}
				}
			}
			if code.Tag == 'r' || code.Tag == 'i' {
				for index := code.J1; index < code.J2; index++ {
					if err := appendLine("+" + current[index]); err != nil {
						return "", err
					}
				}
			}
		}
	}
	return output.String(), nil
}
