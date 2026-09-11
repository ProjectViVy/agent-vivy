package runtime

import (
	"reflect"
	"testing"
)

func TestMCPBackendHasNoLegacyDirectCallMethod(t *testing.T) {
	if _, ok := reflect.TypeOf((*MCPBackend)(nil)).MethodByName("CallTool"); ok {
		t.Fatal("MCPBackend.CallTool remains an executable legacy bypass")
	}
}
