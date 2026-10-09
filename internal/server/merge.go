package server

import (
	"net/http"
	"unicode/utf8"

	diffpreview "ppgit-go/internal/diff"
)

func (s *Server) mergeDiff(r *http.Request) (any, error) {
	payload, err := body(r)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"from_version": true, "to_version": true, "path": true, "expected_from_object": true, "expected_to_object": true, "rows": true}
	for name := range payload {
		if !allowed[name] {
			return nil, bad("Unknown merge request field: " + name)
		}
	}
	values := make(map[string]string, 5)
	for _, name := range []string{"from_version", "to_version", "path", "expected_from_object", "expected_to_object"} {
		value, present := payload[name]
		if !present {
			return nil, bad(name + " is required.")
		}
		maximum := 64
		if name == "path" {
			text, ok := value.(string)
			if !ok || text == "" || utf8.RuneCountInString(text) > 4096 {
				return nil, bad("path must be a nonempty string of at most 4096 characters.")
			}
			values[name] = text
			continue
		}
		if name == "expected_from_object" || name == "expected_to_object" {
			if _, ok := value.(string); !ok {
				return nil, bad(name + " must be a string.")
			}
		}
		text, err := field(map[string]any{name: value}, name, name == "from_version" || name == "to_version" || name == "path", maximum)
		if err != nil {
			return nil, err
		}
		values[name] = text
	}
	list, ok := payload["rows"].([]any)
	if !ok || len(list) == 0 || len(list) > diffpreview.MaxMergeRows {
		return nil, bad("Select between 1 and 10000 changed rows.")
	}
	rows := make([]int, len(list))
	for i, value := range list {
		number, ok := value.(float64)
		if !ok || number < 0 || number >= diffpreview.MaxLines || number != float64(int(number)) {
			return nil, bad("rows must contain nonnegative whole-number diff row indices.")
		}
		rows[i] = int(number)
	}
	result, err := diffpreview.Merge(s.core, r.PathValue("project"), values["from_version"], values["to_version"], values["path"], values["expected_from_object"], values["expected_to_object"], rows)
	s.changed(err)
	return result, err
}
