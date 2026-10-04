// Command go-host is the minimal external VIVY host fixture: it exercises the
// public sdk/host/v1 surface exactly like a desktop embedder would.
package main

import (
	"context"
	"fmt"
	"os"

	host "agent-vivy/sdk/host/v1"
)

func main() {
	// Keep the real Open chain linked: the sealed Generation manifest is
	// retained in the binary only while reachable from live host code.
	if len(os.Args) > 1 {
		_, _ = host.Open(context.Background(), host.Options{ConfigPath: os.Args[1]})
	}
	fmt.Println("go-host fixture:", host.MaxBatchLimit)
}
