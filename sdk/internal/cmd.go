package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Run is the vivy-sdk CLI. It does not start the gateway.
func Run(args []string) int {
	return run(args, os.Stdout, os.Stderr)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: vivy-sdk verify <plugin-dir> | pack --with <plugin> [--out dir] | inspect-artifact <dir>")
		return 2
	}
	switch args[0] {
	case "verify":
		if len(args) != 2 {
			_, _ = fmt.Fprintln(stderr, "usage: vivy-sdk verify <plugin-dir>")
			return 2
		}
		rep, err := Verify(args[1])
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
		if !rep.OK {
			for _, issue := range rep.Issues {
				_, _ = fmt.Fprintln(stderr, issue)
			}
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "ok %s\n", rep.Dir)
		return 0
	case "pack":
		opt, err := parsePackArgs(args[1:])
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
		art, err := Pack(opt)
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		raw, err := json.MarshalIndent(art, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(raw))
		return 0
	case "inspect-artifact":
		if len(args) != 2 {
			_, _ = fmt.Fprintln(stderr, "usage: vivy-sdk inspect-artifact <dir>")
			return 2
		}
		art, err := InspectArtifact(args[1])
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		raw, err := json.MarshalIndent(art, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
		_, _ = fmt.Fprintln(stdout, string(raw))
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "unknown sdk command %q\n", args[0])
		return 2
	}
}
