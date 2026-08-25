# Vivy Sandbox System (D-021)

Vivy's sandbox system provides configurable permission boundaries for agent operations, inspired by the DeepSeek Harness three-tier permission model.

## Overview

The sandbox system controls:
- **Filesystem access**: Read/write permissions based on workspace boundaries
- **Command execution**: Whitelist-based command allowlisting
- **Network access**: Domain whitelisting and private IP blocking
- **Approval workflow**: Configurable approval policies with timeout support

## Sandbox Modes

Three modes govern file and command effects:

### 1. `read_only`
- **File writes**: Denied
- **Command execution**: Denied  
- **File reads**: Allowed within workspace only
- **Use case**: Safe exploration, code review, documentation reading

### 2. `workspace_write` (default)
- **File writes**: Allowed within workspace root
- **Command execution**: Allowed for whitelisted commands
- **File reads**: Allowed within workspace
- **Use case**: Normal development work, safe mutations

### 3. `danger_full_access`
- **File writes**: Unrestricted (but still audited)
- **Command execution**: Relaxed whitelist (dangerous patterns still blocked)
- **File reads**: Unrestricted
- **Use case**: Trusted sessions, system administration tasks
- **Warning**: Use with extreme caution; not recommended for untrusted agents

## Approval Policies

Three policies control when user approval is required:

### 1. `ask` (default)
- All effectful tools require explicit user approval
- Safest option for new or untrusted agents
- User can approve or deny each request

### 2. `never`
- All effectful tools are denied without asking
- Useful for CI/automation or read-only sessions
- No interactive prompts will appear

### 3. `auto`
- Readonly tools auto-execute
- Whitelisted tools auto-execute
- Other effectful tools still require approval
- Balance between safety and convenience

## Configuration

Add to your `config.yaml`:

```yaml
runtime:
  sandbox:
    default_mode: workspace_write
    approval:
      default_policy: ask
      timeout_seconds: 300  # 5 minutes
      auto_approve_tools:
        - read_file
        - search_files
        - list_notes
        - network_search
    network:
      allowed_domains: []  # Empty = no restriction
      deny_private_ips: true
```

## Approval Timeout

Pending approvals automatically expire after the configured timeout:
- Expired approvals are marked as `expired` and denied
- The owning run is cancelled
- A `tool.approval_expired` event is emitted to the Journal
- Background sweeper runs every 10 seconds to clean up expired approvals

## Security Considerations

### Path Traversal Protection
- Symlinks are resolved before validation
- `..` sequences are detected and blocked if they escape workspace
- Absolute paths outside workspace are rejected in confined modes

### Command Injection Prevention
- Shell syntax (`;`, `|`, `&`, `$()`) is forbidden in command names
- Only allowlisted executables can run
- Arguments are passed directly (no shell interpretation)

### Network Isolation
- Private IPs (RFC1918) can be blocked
- Domain whitelist restricts outbound connections
- DNS lookups are performed to validate IP addresses

## API Usage

### Switching Sandbox Mode (Future)
```go
// Service layer will expose:
svc.SetSandboxMode(ctx, sessionID, domain.SandboxModeWorkspaceWrite)
```

### Checking Permissions
```go
// In tool adapters:
if err := sandbox.ValidatePath(path, FileOpWrite); err != nil {
    return fmt.Errorf("sandbox denied: %w", err)
}
```

## Implementation Details

- **SandboxManager**: Core validation engine (immutable, thread-safe)
- **ApprovalScheduler**: Background expiry scanner (10s interval)
- **PolicyEngine**: Evaluates tool calls against policy rules
- **Database**: Stores sandbox mode and approval policy per session/approval

## Migration from Previous Versions

Existing sessions default to `workspace_write` mode and `ask` policy.
No manual migration needed — database schema includes safe defaults.

## References

- DeepSeek Harness Sandbox: `.workspace/deepseek-harness/upstream/docs/subsystems/sandbox.md`
- DeepSeek Harness Approval: `.workspace/deepseek-harness/upstream/docs/subsystems/approval.md`
- Design Document: This implementation follows plan D-021
