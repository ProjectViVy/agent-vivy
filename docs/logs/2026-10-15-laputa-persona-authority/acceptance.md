# Acceptance

1. In a disposable split development instance at `http://127.0.0.1:3015`,
   ask "你是谁？" before initializing Persona. The admitted system instruction
   describes unnamed first-arrival state and includes the Chinese guidance
   "我刚来到世界，我还不太了解自己。" Conversation is admitted normally.
2. Initialize Persona through its page with IDENTITY "你是松柏" and the other
   required documents. Open a new conversation and ask the same question.
   The admitted model input contains the LAPUTA identity, without the default
   Vivy identity or first-arrival instruction. With a compliant model, the
   response uses the saved identity.
3. Change IDENTITY explicitly and start another session: the new session
   receives the new authority while existing frozen sessions retain their
   original identity. Restart preserves those frozen snapshots.
4. An unavailable, corrupt, empty, or conflicting initialized authority
   rejects admission rather than adopting another name. Runtime permission
   rules remain present. Independent coding admission retains its default
   instruction and does not require companion-persona setup.

A deterministic provider verifies request wiring and browser transport, not
the instruction-following quality of an external model. Existing admitted
run snapshots are immutable and are not rewritten by this repair.
