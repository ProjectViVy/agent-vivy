package conformance_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	providerconformance "agent-vivy/sdk/conformance"
	"agent-vivy/sdk/module"
)

func validateReleaseTestOutput(output []byte, goTest bool) error {
	passed := 0
	if goTest {
		scanner := bufio.NewScanner(bytes.NewReader(output))
		scanner.Buffer(make([]byte, 4096), 4<<20)
		for scanner.Scan() {
			var event struct{ Action, Test string }
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				return fmt.Errorf("invalid Go execution evidence: %w", err)
			}
			if event.Test == "" {
				continue
			}
			if event.Action == "skip" || event.Action == "fail" {
				return fmt.Errorf("release test %s did not pass (%s)", event.Test, event.Action)
			}
			if event.Action == "pass" {
				passed++
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
	} else {
		var report struct {
			Success bool `json:"success"`
			Total   int  `json:"numTotalTests"`
			Passed  int  `json:"numPassedTests"`
			Pending int  `json:"numPendingTests"`
		}
		if err := json.Unmarshal(output, &report); err != nil {
			return fmt.Errorf("invalid Vitest execution evidence: %w", err)
		}
		if !report.Success || report.Total != report.Passed || report.Pending != 0 {
			return fmt.Errorf("release Vitest tests did not all pass")
		}
		passed = report.Passed
	}
	if passed == 0 {
		return fmt.Errorf("release command executed no passing tests")
	}
	return nil
}

// The update path accepts observed complete results, never a template of
// passing checks. Any failure leaves the old artifact byte-for-byte intact.
func writeReleaseResults(path string, suites []releaseSuiteCase, actual []providerconformance.ConformanceResult) error {
	canonical, err := providerconformance.CanonicalResults(actual)
	if err != nil {
		return err
	}
	checks := providerconformance.RequiredProviderChecks()
	if len(canonical) != len(suites)*len(checks) {
		return fmt.Errorf("release evidence coverage is incomplete")
	}
	type resultSuite struct {
		Port         string   `json:"port"`
		ProviderID   string   `json:"providerId"`
		EvidenceID   string   `json:"evidenceId"`
		PassedChecks []string `json:"passedChecks"`
	}
	bundle := struct {
		APIVersion     string        `json:"apiVersion"`
		RequiredChecks []string      `json:"requiredChecks"`
		Suites         []resultSuite `json:"suites"`
	}{APIVersion: "vivy.provider-conformance-results/v1", RequiredChecks: checks}
	for _, suite := range suites {
		entry := resultSuite{Port: suite.Port, ProviderID: suite.ProviderID, EvidenceID: suite.EvidenceID}
		for _, check := range checks {
			matched := false
			for _, result := range canonical {
				if result.Port.Port == suite.Port && result.ProviderID == suite.ProviderID && result.EvidenceID == suite.EvidenceID && result.Suite == check && result.Passed {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("release Provider %s lacks passing %s", suite.ProviderID, check)
			}
			entry.PassedChecks = append(entry.PassedChecks, check)
		}
		bundle.Suites = append(bundle.Suites, entry)
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".conformance-results-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(append(data, '\n')); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

func TestReleaseUpdateRequiresCompleteExecutedResults(t *testing.T) {
	suite := releaseSuiteCase{Port: "std/tool@v1", ProviderID: "test/provider", EvidenceID: "test.go#TestProvider"}
	var complete []providerconformance.ConformanceResult
	for _, check := range providerconformance.RequiredProviderChecks() {
		complete = append(complete, providerconformance.ConformanceResult{Port: module.PortRef{Port: suite.Port}, ProviderID: suite.ProviderID, Suite: check, Passed: true, EvidenceID: suite.EvidenceID})
	}
	for _, mode := range []string{"failed", "missing", "foreign", "complete"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "results.json")
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			results := append([]providerconformance.ConformanceResult(nil), complete...)
			switch mode {
			case "failed":
				results[0].Passed = false
			case "missing":
				results = results[1:]
			case "foreign":
				results[0].EvidenceID = "foreign.go#TestOther"
			}
			err := writeReleaseResults(path, []releaseSuiteCase{suite}, results)
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode != "complete" {
				if err == nil || string(got) != "original" {
					t.Fatalf("invalid results published: err=%v content=%s", err, got)
				}
			} else if err != nil || !strings.Contains(string(got), `"providerId": "test/provider"`) {
				t.Fatalf("complete results not published: %v / %s", err, got)
			}
		})
	}
}

func TestReleaseCommandRequiresExecutedTests(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		wantErr      bool
	}{
		{"empty selector", "{\"Action\":\"pass\",\"Package\":\"fixture\"}\n", true},
		{"skipped test", "{\"Action\":\"skip\",\"Test\":\"TestProvider\"}\n", true},
		{"executed test", "{\"Action\":\"pass\",\"Test\":\"TestProvider\"}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateReleaseTestOutput([]byte(tc.output), true); (err != nil) != tc.wantErr {
				t.Fatalf("execution evidence: %v", err)
			}
		})
	}
}
