package records

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

var allowedPatchOps = map[string]struct{}{
	"add":     {},
	"remove":  {},
	"replace": {},
	"move":    {},
	"copy":    {},
	"test":    {},
}

// DefaultMaxTransforms is the default cap on transforms per apply.
const DefaultMaxTransforms = 32

// ApplyTransforms applies RFC 6902 JSON Patch operations to body with sandbox rules.
func ApplyTransforms(body json.RawMessage, transforms json.RawMessage, maxOps int) (json.RawMessage, error) {
	if len(transforms) == 0 || string(transforms) == "null" {
		return body, nil
	}
	if !json.Valid(transforms) {
		return nil, fmt.Errorf("records: transforms: invalid JSON")
	}

	var ops []map[string]any
	if err := json.Unmarshal(transforms, &ops); err != nil {
		return nil, fmt.Errorf("records: transforms: must be array: %w", err)
	}
	if maxOps <= 0 {
		maxOps = DefaultMaxTransforms
	}
	if len(ops) > maxOps {
		return nil, fmt.Errorf("records: transforms: exceeds max %d operations", maxOps)
	}

	doc, err := parseBody(body)
	if err != nil {
		return nil, err
	}

	for i, op := range ops {
		if err := applyOneTransform(doc, op); err != nil {
			return nil, fmt.Errorf("records: transforms[%d]: %w", i, err)
		}
	}

	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("records: transforms: marshal: %w", err)
	}
	return out, nil
}

func parseBody(body json.RawMessage) (map[string]any, error) {
	if len(body) == 0 || string(body) == "null" {
		return map[string]any{}, nil
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("records: transforms: body must be object: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func applyOneTransform(doc map[string]any, op map[string]any) error {
	opName, _ := op["op"].(string)
	if opName == "" {
		return fmt.Errorf("missing op")
	}
	if _, ok := allowedPatchOps[opName]; !ok {
		return fmt.Errorf("unsupported op %q", opName)
	}

	path, _ := op["path"].(string)
	if err := validatePatchPath(path); err != nil {
		return err
	}

	switch opName {
	case "add":
		value, ok := op["value"]
		if !ok {
			return fmt.Errorf("add requires value")
		}
		return patchAdd(doc, path, value)
	case "remove":
		return patchRemove(doc, path)
	case "replace":
		value, ok := op["value"]
		if !ok {
			return fmt.Errorf("replace requires value")
		}
		return patchReplace(doc, path, value)
	case "test":
		value, ok := op["value"]
		if !ok {
			return fmt.Errorf("test requires value")
		}
		return patchTest(doc, path, value)
	case "move":
		from, _ := op["from"].(string)
		if from == "" {
			return fmt.Errorf("move requires from")
		}
		if err := validatePatchPath(from); err != nil {
			return err
		}
		val, err := patchGet(doc, from)
		if err != nil {
			return err
		}
		if err := patchRemove(doc, from); err != nil {
			return err
		}
		return patchAdd(doc, path, val)
	case "copy":
		from, _ := op["from"].(string)
		if from == "" {
			return fmt.Errorf("copy requires from")
		}
		if err := validatePatchPath(from); err != nil {
			return err
		}
		val, err := patchGet(doc, from)
		if err != nil {
			return err
		}
		return patchAdd(doc, path, val)
	default:
		return fmt.Errorf("unsupported op %q", opName)
	}
}

func validatePatchPath(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must start with /")
	}
	if path == "/" {
		return nil
	}
	parts := strings.Split(path, "/")[1:]
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("path contains empty segment")
		}
		if part == ".." {
			return fmt.Errorf("path must not contain parent segments")
		}
		if strings.Contains(part, "~") {
			return fmt.Errorf("path must not contain escapes")
		}
	}
	return nil
}

func parsePath(path string) []string {
	if path == "/" {
		return nil
	}
	return strings.Split(path, "/")[1:]
}

func patchGet(doc map[string]any, path string) (any, error) {
	parts := parsePath(path)
	cur := any(doc)
	for _, part := range parts {
		switch node := cur.(type) {
		case map[string]any:
			val, ok := node[part]
			if !ok {
				return nil, fmt.Errorf("path not found: %s", path)
			}
			cur = val
		case []any:
			idx, dash, err := parseArrayIndex(part, len(node))
			if err != nil {
				return nil, err
			}
			if dash {
				return nil, fmt.Errorf("path not found: %s", path)
			}
			cur = node[idx]
		default:
			return nil, fmt.Errorf("path not found: %s", path)
		}
	}
	return cur, nil
}

func patchAdd(doc map[string]any, path string, value any) error {
	parts := parsePath(path)
	if len(parts) == 0 {
		return fmt.Errorf("cannot add at root")
	}
	parent, last, err := resolveParent(doc, parts)
	if err != nil {
		return err
	}
	switch node := parent.(type) {
	case map[string]any:
		node[last] = value
	case []any:
		if last == "-" {
			arr := append(node, value)
			if err := setAt(doc, parts[:len(parts)-1], arr); err != nil {
				return err
			}
			return nil
		}
		idx, _, err := parseArrayIndex(last, len(node))
		if err != nil {
			return err
		}
		arr := append(node, nil)
		copy(arr[idx+1:], arr[idx:])
		arr[idx] = value
		return setAt(doc, parts[:len(parts)-1], arr)
	default:
		return fmt.Errorf("add target is not object or array")
	}
	return nil
}

func patchRemove(doc map[string]any, path string) error {
	parts := parsePath(path)
	if len(parts) == 0 {
		for k := range doc {
			delete(doc, k)
		}
		return nil
	}
	parent, last, err := resolveParent(doc, parts)
	if err != nil {
		return err
	}
	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last]; !ok {
			return fmt.Errorf("path not found: %s", path)
		}
		delete(node, last)
	case []any:
		idx, dash, err := parseArrayIndex(last, len(node))
		if err != nil {
			return err
		}
		if dash {
			return fmt.Errorf("path not found: %s", path)
		}
		arr := append(node[:idx], node[idx+1:]...)
		return setAt(doc, parts[:len(parts)-1], arr)
	default:
		return fmt.Errorf("remove target is not object or array")
	}
	return nil
}

func patchReplace(doc map[string]any, path string, value any) error {
	parts := parsePath(path)
	if len(parts) == 0 {
		for k := range doc {
			delete(doc, k)
		}
		if m, ok := value.(map[string]any); ok {
			for k, v := range m {
				doc[k] = v
			}
		}
		return nil
	}
	parent, last, err := resolveParent(doc, parts)
	if err != nil {
		return err
	}
	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last]; !ok {
			return fmt.Errorf("path not found: %s", path)
		}
		node[last] = value
	case []any:
		idx, dash, err := parseArrayIndex(last, len(node))
		if err != nil {
			return err
		}
		if dash {
			return fmt.Errorf("path not found: %s", path)
		}
		node[idx] = value
		return setAt(doc, parts[:len(parts)-1], node)
	default:
		return fmt.Errorf("replace target is not object or array")
	}
	return nil
}

func patchTest(doc map[string]any, path string, value any) error {
	got, err := patchGet(doc, path)
	if err != nil {
		return err
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		return err
	}
	wantJSON, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if string(gotJSON) != string(wantJSON) {
		return fmt.Errorf("test failed at %s", path)
	}
	return nil
}

func resolveParent(doc map[string]any, parts []string) (any, string, error) {
	if len(parts) == 0 {
		return nil, "", fmt.Errorf("invalid path")
	}
	cur := any(doc)
	for _, part := range parts[:len(parts)-1] {
		switch node := cur.(type) {
		case map[string]any:
			val, ok := node[part]
			if !ok {
				return nil, "", fmt.Errorf("path not found")
			}
			cur = val
		case []any:
			idx, dash, err := parseArrayIndex(part, len(node))
			if err != nil {
				return nil, "", err
			}
			if dash {
				return nil, "", fmt.Errorf("path not found")
			}
			cur = node[idx]
		default:
			return nil, "", fmt.Errorf("path not found")
		}
	}
	return cur, parts[len(parts)-1], nil
}

func parseArrayIndex(part string, length int) (int, bool, error) {
	if part == "-" {
		return 0, true, nil
	}
	idx, err := strconv.Atoi(part)
	if err != nil {
		return 0, false, fmt.Errorf("invalid array index %q", part)
	}
	if idx < 0 || idx > length {
		return 0, false, fmt.Errorf("array index out of bounds")
	}
	return idx, false, nil
}

func setAt(doc map[string]any, parts []string, value any) error {
	if len(parts) == 0 {
		return fmt.Errorf("invalid path")
	}
	cur := any(doc)
	for i, part := range parts {
		if i == len(parts)-1 {
			switch node := cur.(type) {
			case map[string]any:
				node[part] = value
				return nil
			default:
				return fmt.Errorf("path not found")
			}
		}
		switch node := cur.(type) {
		case map[string]any:
			next, ok := node[part]
			if !ok {
				return fmt.Errorf("path not found")
			}
			cur = next
		case []any:
			idx, dash, err := parseArrayIndex(part, len(node))
			if err != nil {
				return err
			}
			if dash {
				return fmt.Errorf("path not found")
			}
			cur = node[idx]
		default:
			return fmt.Errorf("path not found")
		}
	}
	return nil
}
