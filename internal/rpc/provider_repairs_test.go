package rpc

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"

	"agent-vivy/internal/provider"
)

func TestProviderRepairRPCDefaultProjectionAndFailedRefresh(t *testing.T) {
	data := make([]map[string]string, 2001)
	for i := range data {
		data[i] = map[string]string{"id": fmt.Sprintf("m%d", i)}
	}
	upstream, _ := refreshUpstreamServer(t, http.StatusOK, map[string]any{"data": data})
	probe := &settingsApplierProbe{}
	env, path := newSettingsHandlerEnvWith(t, probe, func(deps *ControlDeps) { deps.ModelLists = &provider.ModelListClient{HTTP: upstream.Client()} })
	result, rpcErr := callControl(t, env.handler, "settings/providers/upsert", map[string]any{"id": "custom-repair", "display_name": "Gateway", "bundle": "openai", "base_url": upstream.URL, "default_model": "preferred", "models": []string{"first"}, "api_key": "test-secret"})
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	entry := result.(providerEntryResult)
	if !reflect.DeepEqual(entry.Models, []string{"first", "preferred"}) {
		t.Error("upsert projection omitted default")
	}
	listed, rpcErr := callControl(t, env.handler, "settings/providers", nil)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !reflect.DeepEqual(listed.(providersResult).Entries[0].Models, entry.Models) {
		t.Error("list projection disagrees with upsert")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	notifications := probe.n
	_, rpcErr = callControl(t, env.handler, "settings/providers/refresh", map[string]any{"id": entry.ID})
	if rpcErr == nil || rpcErr.Code != InternalError {
		t.Errorf("overflow refresh must fail: %v", rpcErr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || probe.n != notifications {
		t.Error("failed refresh changed settings or notified listeners")
	}
}
