// Command vivy-sdk is the author/Studio packer. It is a separate binary
// from the daily vivy.exe species and may grow a bundled toolchain.
package main

import (
	"os"

	sdk "agent-vivy/sdk/internal"
)

func main() {
	os.Exit(sdk.Run(os.Args[1:]))
}
