package divacognitive

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"unicode/utf8"

	action "agent-vivy/sdk/port/controlaction"
)

// Recap schemas are flat objects. Validate the original encoding before Go's
// permissive JSON decoder can normalize invalid UTF-8 or lone UTF-16 surrogates.
// Keys and nulls are checked before typed decode to remove authority ambiguity.
func strictRecapJSON(raw []byte) error {
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return action.ErrInvalidInput
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) || raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return action.ErrInvalidInput
		}
		value, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return action.ErrInvalidInput
		}
		i += 4
		if value >= 0xdc00 && value <= 0xdfff {
			return action.ErrInvalidInput
		}
		if value < 0xd800 || value > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return action.ErrInvalidInput
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return action.ErrInvalidInput
		}
		i += 6
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return action.ErrInvalidInput
	}
	keys := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || keys[key] {
			return action.ErrInvalidInput
		}
		keys[key] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return action.ErrInvalidInput
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return action.ErrInvalidInput
	}
	if _, err = decoder.Token(); err != io.EOF {
		return action.ErrInvalidInput
	}
	return nil
}
