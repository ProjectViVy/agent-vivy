// Command studio-ledger-upgrade is the finite D06 copied-target transition.
// Archive this exact artifact before D14 removes transition sources.
package main

import (
	"agent-vivy/internal/transition/studioledger"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	source := flag.String("source-ledger", "", "quiescent legacy Studio ledger file")
	target := flag.String("target-home", "", "new disposable target Studio home")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	result, err := studioledger.Convert(context.Background(), studioledger.Options{SourceLedger: *source, TargetHome: *target})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
