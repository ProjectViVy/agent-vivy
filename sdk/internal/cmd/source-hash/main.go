package main

import (
	"fmt"
	"os"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: source-hash <directory> <declared-sha256>")
		os.Exit(2)
	}
	digest, err := assemblyv1.HashSourceTree(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(digest)
}
