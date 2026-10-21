package assembly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// ValidateJSONDocument rejects duplicate names and noncanonical struct fields
// before typed decoding. A nil shape retains framing-only validation; callers
// keep their existing typed field diagnostics. Dynamic map keys preserve case.
func ValidateJSONDocument(raw []byte, shape any) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("JSON document must be valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var walk func(reflect.Type) error
	walk = func(expected reflect.Type) error {
		for expected != nil && expected.Kind() == reflect.Pointer {
			expected = expected.Elem()
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			var fields map[string]reflect.Type
			if expected != nil && expected.Kind() == reflect.Struct {
				fields = make(map[string]reflect.Type)
				for _, field := range reflect.VisibleFields(expected) {
					if field.PkgPath != "" {
						continue
					}
					name := strings.Split(field.Tag.Get("json"), ",")[0]
					if name == "-" {
						continue
					}
					if name == "" {
						name = field.Name
					}
					fields[name] = field.Type
				}
			}
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("object key is not a string")
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate JSON key %s", key)
				}
				seen[key] = struct{}{}
				var child reflect.Type
				if fields != nil {
					var exists bool
					child, exists = fields[key]
					if !exists {
						return fmt.Errorf("json: unknown field %q", key)
					}
				} else if expected != nil && expected.Kind() == reflect.Map {
					child = expected.Elem()
				}
				if err := walk(child); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			var child reflect.Type
			if expected != nil && (expected.Kind() == reflect.Slice || expected.Kind() == reflect.Array) {
				child = expected.Elem()
			}
			for decoder.More() {
				if err := walk(child); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
	}
	if err := walk(reflect.TypeOf(shape)); err != nil {
		return err
	}
	return requireManifestEOF(decoder)
}
