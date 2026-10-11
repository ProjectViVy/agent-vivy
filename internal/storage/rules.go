package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"agent-vivy/internal/bundlecontract"
	"agent-vivy/internal/domain"
)

const (
	MaxRunRuleBytes           = 16 << 10
	MaxRunRulesAggregateBytes = 64 << 10
)

var toolkitRuleImportPattern = regexp.MustCompile(`@([a-zA-Z0-9_.~/][a-zA-Z0-9_.~/\-]*)`)

var toolkitRuleImportExtensions = map[string]struct{}{
	".md": {}, ".txt": {}, ".mdx": {}, ".yaml": {}, ".yml": {}, ".json": {}, ".toml": {},
}

// RenderRunRules returns the one synthetic agentsmd file for the ordered,
// already filtered Rule blocks. The format is stable so persisted prompt bytes
// can be checked without consulting the mutable catalog.
func RenderRunRules(blocks []RunRuleBlock) string {
	var out strings.Builder
	out.WriteString("# Captured Vivy Toolkit Rules\n\n")
	out.WriteString("These captured bundle rules provide project guidance. Vivy's system and runtime policy, current user request, and explicit governance determine priorities, capabilities, and approvals.\n")
	for _, block := range blocks {
		fmt.Fprintf(&out, "\n## Toolkit rule: %s/%s\n\n", block.Release.BundleID, block.Ref.LocalID)
		out.WriteString(block.Body)
		if !strings.HasSuffix(block.Body, "\n") {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// RunRulesDigest computes the digest over the exact synthesized body, face,
// and per-component provenance. Digest itself is excluded from its input.
func RunRulesDigest(in RunRulesSnapshot) (string, error) {
	canonical, err := bundlecontract.CanonicalJSON(struct {
		Face   string         `json:"face"`
		Blocks []RunRuleBlock `json:"blocks"`
		Body   string         `json:"body"`
	}{Face: in.Face, Blocks: in.Blocks, Body: in.Body})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// ValidateRunRulesSnapshot validates the bounded literal Rules projection.
// Its body can therefore be handed to Eino agentsmd without enabling @imports
// or reading mutable project files on behalf of a bundle.
func ValidateRunRulesSnapshot(in *RunRulesSnapshot) error {
	if in == nil || !domain.Face(in.Face).Valid() || len(in.Blocks) == 0 || in.Body == "" || !bundlecontract.ValidDigest(in.Digest) {
		return errors.New("storage: invalid Toolkit Rules snapshot shape")
	}
	if !utf8.ValidString(in.Body) || strings.ContainsRune(in.Body, '\x00') || in.Body != RenderRunRules(in.Blocks) {
		return errors.New("storage: Toolkit Rules body is not the canonical literal projection")
	}
	total := 0
	previousID := ""
	seen := make(map[bundlecontract.ComponentRef]struct{}, len(in.Blocks))
	for _, block := range in.Blocks {
		if block.Ref.Kind != bundlecontract.Rule || !bundlecontract.ValidInstallID(block.Ref.InstallID) || !bundlecontract.ValidLocalID(block.Ref.LocalID) ||
			block.Release.InstallID != block.Ref.InstallID || !bundlecontract.ValidInstallID(block.Release.InstallID) || !bundlecontract.ValidBundleID(block.Release.BundleID) ||
			!bundlecontract.ValidDigest(block.Release.ReleaseDigest) || block.Release.ReleaseSequence <= 0 || !bundlecontract.ValidDigest(block.ContentDigest) {
			return errors.New("storage: invalid Toolkit Rule provenance")
		}
		projectedID, err := bundlecontract.SkillID(block.Ref.InstallID, block.Ref.LocalID)
		if err != nil || (previousID != "" && previousID >= projectedID) {
			return errors.New("storage: Toolkit Rule blocks are not uniquely sorted")
		}
		previousID = projectedID
		if _, duplicate := seen[block.Ref]; duplicate {
			return errors.New("storage: duplicate Toolkit Rule block")
		}
		seen[block.Ref] = struct{}{}
		bodyBytes := len([]byte(block.Body))
		if bodyBytes == 0 || bodyBytes > MaxRunRuleBytes || !utf8.ValidString(block.Body) || strings.ContainsRune(block.Body, '\x00') || hasToolkitRuleImport(block.Body) {
			return errors.New("storage: Toolkit Rule body is invalid, oversized, or imports another path")
		}
		total += bodyBytes
		if total > MaxRunRulesAggregateBytes {
			return errors.New("storage: aggregate Toolkit Rules exceed the byte limit")
		}
	}
	if expectedBody := RenderRunRules(in.Blocks); expectedBody != in.Body {
		return errors.New("storage: Toolkit Rules projection does not match its blocks")
	}
	digest, err := RunRulesDigest(*in)
	if err != nil || digest != in.Digest {
		return errors.New("storage: Toolkit Rules digest does not match its content")
	}
	return nil
}

func hasToolkitRuleImport(body string) bool {
	for _, match := range toolkitRuleImportPattern.FindAllStringSubmatch(body, -1) {
		path := match[1]
		if strings.Contains(path, "/") {
			return true
		}
		if _, ok := toolkitRuleImportExtensions[filepath.Ext(path)]; ok {
			return true
		}
	}
	return false
}
