package bundlecontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf8"
)

// CloneManifest gives owners an independent typed copy, including maps/slices.
func CloneManifest(m Manifest) Manifest {
	b, _ := json.Marshal(m)
	var out Manifest
	_ = json.Unmarshal(b, &out)
	return normalize(out)
}
func CanonicalManifest(m Manifest) ([]byte, error) {
	if err := ValidateManifest(m, DefaultLimits()); err != nil {
		return nil, err
	}
	m = CloneManifest(m)
	sort.Slice(m.Components.Skills, func(i, j int) bool { return m.Components.Skills[i].ID < m.Components.Skills[j].ID })
	sort.Slice(m.Components.Masks, func(i, j int) bool { return m.Components.Masks[i].ID < m.Components.Masks[j].ID })
	sort.Slice(m.Components.Rules, func(i, j int) bool { return m.Components.Rules[i].ID < m.Components.Rules[j].ID })
	sort.Slice(m.Components.Hooks, func(i, j int) bool { return m.Components.Hooks[i].ID < m.Components.Hooks[j].ID })
	sort.Slice(m.Components.MCP, func(i, j int) bool { return m.Components.MCP[i].ID < m.Components.MCP[j].ID })
	for i := range m.Components.Masks {
		sort.Strings(m.Components.Masks[i].RecommendedSkills)
	}
	for i := range m.Components.Rules {
		sort.Strings(m.Components.Rules[i].Faces)
	}
	sort.Strings(m.Resources)
	sort.Strings(m.LicenseFiles)
	return CanonicalJSON(m)
}

// CanonicalJSON is the versioned string/integer encoding shared by manifest,
// inventory and operation input digests. Ordering of arrays is preserved.
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, Invalid("canonical_json")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var tree any
	if err := d.Decode(&tree); err != nil {
		return nil, Invalid("canonical_json")
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, tree); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func writeCanonical(out *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		out.WriteString(strconv.FormatBool(x))
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return Invalid("integer")
		}
		out.WriteString(strconv.FormatInt(n, 10))
	case string:
		if !utf8.ValidString(x) {
			return Invalid("encoding")
		}
		out.WriteByte('"')
		for _, r := range x {
			switch {
			case r == '"' || r == '\\':
				out.WriteByte('\\')
				out.WriteRune(r)
			case r < 0x20:
				fmt.Fprintf(out, "\\u%04x", r)
			default:
				out.WriteRune(r)
			}
		}
		out.WriteByte('"')
	case []any:
		out.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, k); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := writeCanonical(out, x[k]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return Invalid("canonical_json")
	}
	return nil
}

func ArtifactDigest(m Manifest, inventory []InventoryEntry) (string, error) {
	manifest, err := CanonicalManifest(m)
	if err != nil {
		return "", err
	}
	paths := DeclaredPaths(m)
	if len(inventory) != len(paths) {
		return "", Invalid("inventory")
	}
	entries := append([]InventoryEntry{}, inventory...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var total int64
	for i, e := range entries {
		if e.Path != paths[i] || e.Size < 0 || e.Size > DefaultLimits().ResourceBytes {
			return "", Invalid("inventory")
		}
		b, err := hex.DecodeString(e.SHA256)
		if err != nil || len(b) != sha256.Size || hex.EncodeToString(b) != e.SHA256 {
			return "", Invalid("inventory_digest")
		}
		total += e.Size
	}
	if total > DefaultLimits().TotalBytes {
		return "", Invalid("total_bytes")
	}
	b, err := CanonicalJSON(entries)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte("vivy.bundle.digest/v1"))
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(manifest)))
	h.Write(size[:])
	h.Write(manifest)
	binary.BigEndian.PutUint64(size[:], uint64(len(b)))
	h.Write(size[:])
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validDigest(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == sha256.Size && hex.EncodeToString(b) == s
}
