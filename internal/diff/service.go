package diff

import (
	"errors"
	"strings"
	"unicode/utf8"

	"ppgit-go/internal/core"
)

func GetDiff(c *core.Core, projectID, versionA, versionB, filePath string, includeDetails, includePatch bool) (map[string]any, error) {
	if filePath != "" {
		if err := core.ValidatePath(filePath); err != nil {
			return nil, err
		}
	}
	leftVersion, left, err := c.ReadVersionSnapshot(projectID, versionA)
	if err != nil {
		return nil, err
	}
	rightVersion, right, err := c.ReadVersionSnapshot(projectID, versionB)
	if err != nil {
		return nil, err
	}
	if sequence(leftVersion["sequence"]) > sequence(rightVersion["sequence"]) {
		leftVersion, rightVersion = rightVersion, leftVersion
		left, right = right, left
	}
	files := make([]map[string]any, 0)
	skipped := make([]string, 0)
	budget := NewBudget()
	compareLeft, compareRight := left, right
	if filePath != "" {
		compareLeft, compareRight = core.Snapshot{}, core.Snapshot{}
		if entry, ok := left[filePath]; ok {
			compareLeft[filePath] = entry
		}
		if entry, ok := right[filePath]; ok {
			compareRight[filePath] = entry
		}
	}
	changes := core.DiffSnapshots(compareLeft, compareRight)
	for _, change := range changes {
		path, _ := change["path"].(string)
		if filePath != "" && path != filePath {
			continue
		}
		leftEntry, hasLeft := left[path]
		rightEntry, hasRight := right[path]
		size := max(leftEntry.Size, rightEntry.Size)
		item := map[string]any{
			"path": path, "change_type": change["status"], "too_large": false,
			"message": nil, "patch": nil, "size": size,
		}
		item["mergeable"] = false
		item["from_object_id"], item["to_object_id"] = leftEntry.ObjectID, rightEntry.ObjectID
		item["merge_disabled_code"], item["merge_disabled_reason"] = "preview_unavailable", "\u5f53\u524d\u6587\u4ef6\u6ca1\u6709\u53ef\u5408\u5e76\u7684\u5b8c\u6574\u6587\u672c\u5dee\u5f02"
		files = append(files, item)
		if size > core.MaxPreviewSize {
			item["too_large"] = true
			item["message"] = "[\u6587\u4ef6\u8fc7\u5927 (>10MB)\uff0c\u8df3\u8fc7\u5dee\u5f02\u9884\u89c8]"
			skipped = append(skipped, path)
			continue
		}
		if !includeDetails && !includePatch {
			continue
		}
		if !leftEntry.IsText && !rightEntry.IsText {
			item["message"] = "[\u975e\u6587\u672c\u6587\u4ef6\uff0c\u65e0\u6cd5\u9884\u89c8\u5dee\u5f02]"
			continue
		}
		if includeDetails {
			item["diff_rows"] = nil
		}

		limited := func() {
			item["preview_limited"] = true
			item["message"] = LimitMessage
			skipped = append(skipped, path)
		}
		if err := budget.ReserveInput(leftEntry.Size + rightEntry.Size); err != nil {
			limited()
			continue
		}
		validText := true

		readText := func(entry core.Entry, exists bool) (string, error) {
			if !exists || !entry.IsText {
				return "", nil
			}
			data, err := c.ReadObject(projectID, entry.ObjectID, MaxInputBytes)
			if err != nil {
				return "", err
			}
			validText = validText && utf8.Valid(data)
			return strings.ToValidUTF8(string(data), "\ufffd"), nil
		}
		oldText, err := readText(leftEntry, hasLeft)
		if err != nil {
			if errors.Is(err, core.ErrTooLarge) {
				limited()
				continue
			}
			return nil, err
		}
		newText, err := readText(rightEntry, hasRight)
		if err != nil {
			if errors.Is(err, core.ErrTooLarge) {
				limited()
				continue
			}
			return nil, err
		}
		preview, err := buildPreviewReserved(path, oldText, newText, includeDetails, includePatch, budget)
		if err != nil {
			if errors.Is(err, ErrBudgetExceeded) {
				limited()
				continue
			}
			return nil, err
		}
		if preview.Patch != nil {
			item["patch"] = *preview.Patch
		} else if includePatch {
			item["message"] = "[\u6ca1\u6709\u53ef\u7528\u5dee\u5f02\u9884\u89c8]"
		}
		if includeDetails {
			item["diff_rows"] = preview.Rows
			code, reason := mergeAvailability(leftVersion, rightVersion, leftEntry, rightEntry, hasLeft, hasRight)
			if !validText {
				code, reason = "invalid_encoding", "\u4ec5\u652f\u6301\u5408\u5e76 UTF-8 \u6587\u672c\u6587\u4ef6"
			}
			changed := false
			for _, row := range preview.Rows {
				changed = changed || row.Status != "same"
			}
			if !changed && code == "" {
				code, reason = "no_changed_rows", "\u6ca1\u6709\u53ef\u5408\u5e76\u7684\u5dee\u5f02\u884c"
			}
			item["mergeable"] = code == ""
			item["merge_disabled_code"], item["merge_disabled_reason"] = code, reason
		}
	}
	return map[string]any{
		"from_version": leftVersion, "to_version": rightVersion,
		"files": files, "skipped_files": skipped,
	}, nil
}

func sequence(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}
