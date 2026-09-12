package loop

import (
	"context"
	"io"
	"strings"
	"testing"

	"agent-vivy/internal/runtime"
	"agent-vivy/internal/testsupport"
	"agent-vivy/internal/tools"
)

func TestLoopDriverRejectsMissingFactory(t *testing.T) {
	if _, err := Compose(nil); err == nil {
		t.Fatal("Compose(nil) succeeded")
	}
}

func TestLoopDriverPreservesPinnedEinoStreamingPath(t *testing.T) {
	active, err := tools.Builtin(nil).Resolve([]string{tools.EchoInfoName})
	if err != nil {
		t.Fatal(err)
	}
	driver, err := Compose(runtime.NewEngineFactory(runtime.WrapModel(testsupport.NewEchoModel())))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := driver.Build(context.Background(), active, runtime.EngineConfig{
		StreamBuffer: 8, MaxEventPayloadBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	iterator := engine.Query(context.Background(), "through loop module")
	var chunks []string
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			t.Fatal(event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil || event.Output.MessageOutput.MessageStream == nil {
			continue
		}
		for {
			chunk, recvErr := event.Output.MessageOutput.MessageStream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				t.Fatal(recvErr)
			}
			chunks = append(chunks, chunk.Content)
		}
	}
	if got, want := strings.Join(chunks, ""), "test response to: through loop module"; got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}

func TestLoopModuleOwnsCanonicalCorePort(t *testing.T) {
	descriptor := NewModule().Descriptor()
	if descriptor.Module.ID != ID || len(descriptor.Provides) != 1 || descriptor.Provides[0].Port != Port {
		t.Fatalf("descriptor = %#v", descriptor)
	}
}
