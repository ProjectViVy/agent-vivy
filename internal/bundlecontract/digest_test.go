package bundlecontract

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func inventoryBody(path, body string) InventoryEntry {
	h := sha256.Sum256([]byte(body))
	return InventoryEntry{Path: path, Size: int64(len(body)), SHA256: hex.EncodeToString(h[:])}
}
func TestArtifactDigestGolden(t *testing.T) {
	m, _ := DecodeManifest([]byte(minimal))
	want := `{"apiVersion":"vivy.bundle/v1","components":{"hooks":[],"masks":[],"mcp":[],"rules":[],"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}]},"id":"example/experts","licenseFiles":[],"name":"Experts","resources":[],"version":"1"}`
	b, err := CanonicalManifest(m)
	if err != nil || string(b) != want {
		t.Fatalf("%s %v", b, err)
	}
	i := []InventoryEntry{inventoryBody("skills/plan/SKILL.md", "body")}
	d, err := ArtifactDigest(m, i)
	if err != nil || d != "ac57d0d77f4eedfd11049d85a9f87be5c4b49d030cfd98c94a2755ea0f96a64b" {
		t.Fatal(d, err)
	}
	m2, err := DecodeManifest([]byte("\n " + minimal + " \n"))
	if err != nil {
		t.Fatal(err)
	}
	d2, _ := ArtifactDigest(m2, i)
	if d != d2 {
		t.Fatal("format affected digest")
	}
	i[0] = inventoryBody(i[0].Path, "Body")
	d2, _ = ArtifactDigest(m, i)
	if d == d2 {
		t.Fatal("content did not affect digest")
	}
	m.Components.MCP = []MCPEntry{{ID: "server", Command: "server", Args: []string{"one", "two"}}}
	d, _ = ArtifactDigest(m, i)
	m.Components.MCP[0].Args = []string{"two", "one"}
	d2, _ = ArtifactDigest(m, i)
	if d == d2 {
		t.Fatal("semantic args order lost")
	}
	_, err = ArtifactDigest(m, append(i, i[0]))
	requireCode(t, err, "invalid_manifest")
	_, err = ArtifactDigest(m, nil)
	requireCode(t, err, "invalid_manifest")
}
func TestCanonicalStringsAndSets(t *testing.T) {
	m, _ := DecodeManifest([]byte(minimal))
	m.Name = "中国<&/\n\t\"\\"
	b, err := CanonicalManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `中国<&/\u000a\u0009\"\\`) {
		t.Fatal(string(b))
	}
	m.Resources = []string{"z", "a"}
	b, _ = CanonicalManifest(m)
	if strings.Index(string(b), `"a"`) > strings.Index(string(b), `"z"`) {
		t.Fatal("not sorted")
	}
	if m.Resources[0] != "z" {
		t.Fatal("mutated input")
	}
}
