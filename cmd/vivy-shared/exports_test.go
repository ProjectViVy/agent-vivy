package main

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"agent-vivy/internal/embedded"
)

// TestABIHeaderMatchesGoVersion keeps the published C constant and the Go
// constant in lockstep: vivy_abi.h is what the Rust bridge asserts against.
func TestABIHeaderMatchesGoVersion(t *testing.T) {
	body, err := os.ReadFile("vivy_abi.h")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`#define\s+VIVY_ABI_VERSION\s+([0-9]+)`).FindSubmatch(body)
	if match == nil {
		t.Fatal("vivy_abi.h does not define VIVY_ABI_VERSION")
	}
	if got, _ := strconv.Atoi(string(match[1])); got != embedded.ABIVersion {
		t.Fatalf("VIVY_ABI_VERSION %d, Go abiVersion %d", got, embedded.ABIVersion)
	}
}
