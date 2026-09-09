# Plugin I18N Contract Design Verification

## Commands

### Full repository gate

```text
just ci
```

Result: **BLOCKED BY ENVIRONMENT**. The command exited 127 because `just` is
not installed in the current execution environment:

```text
/bin/bash: line 1: just: command not found
```

This design-only delivery does not claim that `just ci` passed.

### Spec self-review

The design was checked for:

- unresolved `TBD`, `TODO`, and `FIXME` placeholders;
- contradictory descriptor, fallback, evidence, and hashing rules;
- ambiguous phase scheduling or implementation claims;
- accidental runtime, SDK, or active-locale scope expansion;
- whitespace errors with `git diff --check` after staging.

No unresolved design placeholder remains. The only use of
`non-normative` describes the pre-freeze repository state, and the only use of
`TODO` names the repository board that the later implementation must align.
