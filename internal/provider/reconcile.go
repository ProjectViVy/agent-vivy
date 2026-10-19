package provider

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ReconcileAdapters proves that the embedded provider data and the sealed
// adapter set agree in both directions (decision D15, DESIGN.md §3.4 rules
// 12-13). It is called once at startup, before the model resolver is built,
// and a mismatch is a startup failure: fail closed rather than let a data
// edit quietly change which wire protocols this build can speak.
//
// Both directions are mandatory: every family the data declares must be
// executable, and every executable family must be reachable from the data.
func ReconcileAdapters(vendors []Vendor, sealedAdapters []string) error {
	declared := make(map[string]struct{})
	for _, vendor := range vendors {
		for _, endpoint := range vendor.Endpoints {
			declared[endpoint.Adapter] = struct{}{}
		}
	}
	sealed := make(map[string]struct{}, len(sealedAdapters))
	for _, adapter := range sealedAdapters {
		if strings.TrimSpace(adapter) == "" {
			continue
		}
		sealed[adapter] = struct{}{}
	}

	var errs []error
	for _, adapter := range sortedKeys(declared) {
		if _, ok := sealed[adapter]; !ok {
			errs = append(errs, fmt.Errorf("provider data: adapter %q is not sealed by this build (sealed: %s)", adapter, strings.Join(sortedKeys(sealed), ", ")))
		}
	}
	for _, adapter := range sortedKeys(sealed) {
		if _, ok := declared[adapter]; !ok {
			errs = append(errs, fmt.Errorf("sealed adapter %q has no endpoint in the provider data", adapter))
		}
	}
	return errors.Join(errs...)
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
