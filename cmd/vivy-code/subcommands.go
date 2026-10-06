package main

// Subcommand surface: `vivy-code mcp …` and `vivy-code config get|set` —
// operator actions that must work without entering the TUI (pi parity).
// Writes go through the same settings overlay document the UI owns
// (settings.yaml via internal/app/settings) — never a second writer.
// `mcp login` is deferred by spec O2 (OAuth lands via a provider plugin).

import (
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"agent-vivy/internal/app/settings"
	"agent-vivy/internal/config"
	"gopkg.in/yaml.v3"
)

// runSubcommand returns handled=true when argv names a subcommand; the
// returned code is the process exit status.
func runSubcommand(args []string, cfg config.Config, out, errw io.Writer) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}
	switch args[0] {
	case "mcp":
		return true, mcpSubcommand(args[1:], cfg, out, errw)
	case "config":
		return true, configSubcommand(args[1:], cfg, out, errw)
	}
	return false, 0
}

// ---------- mcp ----------

type mcpRow struct {
	name      string
	transport string
	enabled   bool
	deferred  string
}

func mcpSubcommand(args []string, cfg config.Config, out, errw io.Writer) int {
	path := settings.Path(cfg.DataDirectory())
	fail := func(err error) int {
		fmt.Fprintf(errw, "vivy-code mcp: %v\n", err)
		return 1
	}
	usage := func() int {
		fmt.Fprint(errw, `usage:
  vivy-code mcp list                    configured servers + transport
  vivy-code mcp status                  probe endpoint/PATH reachability
  vivy-code mcp add <name> --endpoint <url>
  vivy-code mcp add <name> --command <exe> [args...]
  vivy-code mcp remove <name>
  vivy-code mcp login <name>            deferred (OAuth is plugin scope, spec O2)
`)
		return 2
	}
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "list":
		rows, err := mcpRows(path, cfg)
		if err != nil {
			return fail(err)
		}
		if len(rows) == 0 {
			fmt.Fprintln(out, "(no mcp servers configured)")
			return 0
		}
		for _, r := range rows {
			enabled := "enabled"
			if !r.enabled {
				enabled = "disabled"
			}
			if r.deferred != "" {
				enabled = "deferred: " + r.deferred
			}
			fmt.Fprintf(out, "%-24s %-10s %s\n", r.name, r.transport, enabled)
		}
		return 0
	case "status":
		rows, err := mcpRows(path, cfg)
		if err != nil {
			return fail(err)
		}
		for _, r := range rows {
			fmt.Fprintf(out, "%-24s %s\n", r.name, probeMCP(r))
		}
		return 0
	case "add":
		entry, err := parseMCPAdd(args[1:])
		if err != nil {
			fmt.Fprintf(errw, "vivy-code mcp add: %v\n", err)
			return usage()
		}
		_, err = settings.Update(path, func(s settings.Settings) (settings.Settings, error) {
			return s.UpsertMCPServer(entry), nil
		})
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "added %s\n", entry.Name)
		return 0
	case "remove":
		if len(args) != 2 {
			return usage()
		}
		found := false
		_, err := settings.Update(path, func(s settings.Settings) (settings.Settings, error) {
			next, ok := s.DeleteMCPServer(args[1])
			found = ok
			return next, nil
		})
		if err != nil {
			return fail(err)
		}
		if !found {
			return fail(fmt.Errorf("no server named %q", args[1]))
		}
		fmt.Fprintf(out, "removed %s\n", args[1])
		return 0
	case "login":
		name := ""
		if len(args) > 1 {
			name = " for " + args[1]
		}
		fmt.Fprintf(out, "mcp login%s is deferred: OAuth lands via a provider plugin (spec O2)\n", name)
		return 0
	}
	return usage()
}

// mcpRows returns the effective server list: the settings overlay when the
// operator owns it, else the config.yaml default.
func mcpRows(settingsPath string, cfg config.Config) ([]mcpRow, error) {
	doc, err := settings.Load(settingsPath)
	if err != nil {
		return nil, err
	}
	var rows []mcpRow
	if doc.MCPServers != nil {
		for _, s := range doc.MCPServersOrEmpty() {
			rows = append(rows, mcpRow{
				name:      s.Name,
				transport: mcpTransport(s.Endpoint, s.Command, s.Args),
				enabled:   s.Enabled == nil || *s.Enabled,
				deferred:  s.DeferredReason,
			})
		}
		return rows, nil
	}
	for _, s := range cfg.Runtime.MCPServers {
		rows = append(rows, mcpRow{
			name:      s.Name,
			transport: mcpTransport(s.Endpoint, s.Command, s.Args),
			enabled:   s.Enabled == nil || *s.Enabled,
			deferred:  s.DeferredReason,
		})
	}
	return rows, nil
}

func mcpTransport(endpoint, command string, args []string) string {
	if endpoint != "" {
		return endpoint
	}
	return strings.TrimSpace(command + " " + strings.Join(args, " "))
}

// parseMCPAdd parses `mcp add <name> (--endpoint url | --command exe [args...])`.
// Everything after --command's first token belongs to the child argv so
// `mcp add x --command npx -y foo` keeps `-y foo` (pi parity).
func parseMCPAdd(args []string) (settings.MCPServer, error) {
	if len(args) < 3 || args[1] != "--endpoint" && args[1] != "--command" {
		return settings.MCPServer{}, fmt.Errorf("expected: add <name> --endpoint <url> | --command <exe> [args...]")
	}
	srv := settings.MCPServer{Name: strings.TrimSpace(args[0])}
	switch args[1] {
	case "--endpoint":
		if len(args) != 3 {
			return srv, fmt.Errorf("--endpoint takes exactly one value")
		}
		srv.Endpoint = args[2]
	case "--command":
		srv.Command = args[2]
		if len(args) > 3 {
			srv.Args = append([]string(nil), args[3:]...)
		}
	}
	return srv, nil
}

// probeMCP answers "is it reachable at all" for the list surface — an HTTP
// GET for remote endpoints, exec.LookPath for stdio commands. It does not
// open a session; the live circuit state lives in the running host.
func probeMCP(r mcpRow) string {
	parts := strings.Fields(r.transport)
	if len(parts) == 0 {
		return "unconfigured"
	}
	if strings.HasPrefix(parts[0], "http://") || strings.HasPrefix(parts[0], "https://") {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(parts[0])
		if err != nil {
			return "unreachable: " + err.Error()
		}
		resp.Body.Close()
		return fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
	}
	if _, err := exec.LookPath(parts[0]); err != nil {
		return "executable not found: " + parts[0]
	}
	return "executable ok"
}

// ---------- config ----------

// secretPathSegment rejects write access to secret-bearing leaves. Env-var
// references (*_env, env_from) are references, not values — allowed.
func secretPathSegment(seg string) bool {
	if strings.HasSuffix(seg, "_env") || seg == "env_from" {
		return false
	}
	return seg == "api_key" || strings.HasSuffix(seg, "_key") ||
		strings.HasSuffix(seg, "_secret") || strings.HasSuffix(seg, "_token")
}

func configSubcommand(args []string, cfg config.Config, out, errw io.Writer) int {
	path := settings.Path(cfg.DataDirectory())
	fail := func(err error) int {
		fmt.Fprintf(errw, "vivy-code config: %v\n", err)
		return 1
	}
	usage := func() int {
		fmt.Fprint(errw, `usage:
  vivy-code config get <path>            e.g. provider, default_model, compaction.enabled
  vivy-code config set <path> <value>    schema-validated write to settings.yaml
  paths are settings.yaml keys (dotted for nesting); secret leaves are refused
`)
		return 2
	}
	if len(args) < 2 {
		return usage()
	}
	verb, dotted := args[0], args[1]
	segs := strings.Split(dotted, ".")
	for _, s := range segs {
		if secretPathSegment(s) {
			fmt.Fprintf(errw, "vivy-code config: %q is a secret path; keep credentials in env vars (D-010)\n", dotted)
			return 2
		}
	}
	switch verb {
	case "get":
		if len(args) != 2 {
			return usage()
		}
		doc, err := settings.Load(path)
		if err != nil {
			return fail(err)
		}
		val, ok, err := settingsGetPath(doc, segs)
		if err != nil {
			return fail(err)
		}
		if !ok {
			return fail(fmt.Errorf("unknown config path %q", dotted))
		}
		data, _ := yaml.Marshal(val)
		fmt.Fprintf(out, "%s: %s", dotted, data)
		return 0
	case "set":
		if len(args) != 3 {
			return usage()
		}
		_, err := settings.Update(path, func(s settings.Settings) (settings.Settings, error) {
			next, err := settingsSetPath(s, segs, args[2])
			if err != nil {
				return s, err
			}
			return next, nil
		})
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "%s = %s\n", dotted, args[2])
		return 0
	}
	return usage()
}

// settingsAsMap round-trips the document through its YAML form so dotted
// paths address real schema keys (and unknown paths fail instead of writing
// garbage fields).
func settingsAsMap(s settings.Settings) (map[string]any, error) {
	data, err := yaml.Marshal(s)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func mapAsSettings(m map[string]any) (settings.Settings, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return settings.Settings{}, err
	}
	var s settings.Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return settings.Settings{}, err
	}
	return s, nil
}

func settingsGetPath(s settings.Settings, segs []string) (any, bool, error) {
	m, err := settingsAsMap(s)
	if err != nil {
		return nil, false, err
	}
	var cur any = m
	for _, seg := range segs {
		next, ok := cur.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		cur, ok = next[seg]
		if !ok {
			return nil, false, nil
		}
	}
	return cur, true, nil
}

// settingsSetPath writes one scalar leaf. The path must already exist in the
// marshaled schema (unknown keys are refused), and the new value keeps the
// leaf's scalar kind (string/int/bool) so typed fields can't be corrupted.
func settingsSetPath(s settings.Settings, segs []string, raw string) (settings.Settings, error) {
	m, err := settingsAsMap(s)
	if err != nil {
		return s, err
	}
	cur := m
	for i, seg := range segs {
		next, ok := cur[seg]
		if !ok {
			return s, fmt.Errorf("unknown config path %q", strings.Join(segs[:i+1], "."))
		}
		if i == len(segs)-1 {
			cur[seg] = coerceScalar(next, raw)
			break
		}
		sub, ok := next.(map[string]any)
		if !ok {
			return s, fmt.Errorf("%q is a leaf, not a section", strings.Join(segs[:i+1], "."))
		}
		cur = sub
	}
	return mapAsSettings(m)
}

// coerceScalar parses raw into the existing leaf's kind; strings stay
// strings. Bare scalars with no existing leaf never reach here.
func coerceScalar(existing any, raw string) any {
	switch existing.(type) {
	case bool:
		return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
	case int, int64:
		var n int64
		if _, err := fmt.Sscanf(raw, "%d", &n); err == nil {
			return n
		}
		return raw // invalid → schema validation reports it
	default:
		return raw
	}
}
