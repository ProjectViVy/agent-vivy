package provider

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestProviderRepairDiscoveryBounds(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ids, want []string
		fail      bool
	}{
		{name: "byte boundaries", ids: []string{" " + strings.Repeat("a", 256) + " ", strings.Repeat("b", 257), strings.Repeat("界", 85) + "x", strings.Repeat("界", 85) + "xx", "ok", " ok ", " "}, want: []string{strings.Repeat("a", 256), strings.Repeat("界", 85) + "x", "ok"}},
		{name: "inclusive limit", ids: repairModelIDs(2000), want: repairModelIDs(2000)},
		{name: "overflow", ids: repairModelIDs(2001), fail: true},
		{name: "count accepted only", ids: append(repairModelIDs(2000), "m0", " ", strings.Repeat("x", 257)), want: repairModelIDs(2000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]map[string]string, 0, len(tc.ids))
			for _, id := range tc.ids {
				data = append(data, map[string]string{"id": id})
			}
			ts, _, _ := testModelsServer(t, http.StatusOK, map[string]any{"data": data})
			got, err := (&ModelListClient{HTTP: ts.Client()}).List(context.Background(), ts.URL, "test-secret")
			if tc.fail {
				if err == nil || got != nil {
					t.Error("overflow must fail without partial results")
				}
				if err != nil && (!strings.Contains(err.Error(), "2000") || strings.Contains(err.Error(), "test-secret")) {
					t.Errorf("unsafe or missing limit diagnostic: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("accepted %d IDs, want %d valid bounded IDs", len(got), len(tc.want))
			}
		})
	}
}
func repairModelIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("m%d", i)
	}
	return ids
}
