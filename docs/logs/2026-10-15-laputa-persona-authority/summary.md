# LAPUTA persona admission

The primary prompt composer injected the default "You are Vivy" persona and
its immutable-identity rule alongside LAPUTA FrozenCore. The development
Journal confirmed that the saved "你是松柏" identity reached admission while
the contradictory default identity remained in the same instruction.

FrozenCore now supplies the sole persona authority. Its bytes and source
evidence remain in the existing Frozen payload; the unused Persona value is
empty. Two explicit authorities cannot be composed. Runtime permissions,
mask data, and session freezing remain on the existing admission path.

The owner's clarified requirement also permits conversation before persona
initialization. Only the explicit uninitialized status installs unnamed
first-arrival guidance, including "我刚来到世界，我还不太了解自己。" in Chinese.
Initialization replaces that guidance with LAPUTA authority on subsequent
admissions. Other preparation errors and empty initialized projections fail
closed. The coding face and compositions without cognitive authority retain
their existing default behavior. The Persona module's English/Chinese
initialization hint now describes defining identity rather than claiming
conversation is blocked before initialization; its existing UI Port, controls,
and Recipe selection are unchanged. No production Journal, authority documents,
credentials, migrations, or providers are changed. The SDK regenerated the
Assembly projection for the changed Persona catalog only.

The full gate also exposed a transient-state polling failure in the existing
gateway-less approval-expiry test. It now observes the durable request and
expiry events instead of requiring a poll to catch a 400 ms pending window.
Production approval lifetime and scheduler behavior are unchanged.

## Design and Eino capability check

Choose mutually exclusive persona selection at the existing Service.Run
admission and immutable prompt composer. Appending a priority disclaimer would
retain two competing identity sources and unnecessary prompt bytes. Inspected
the pinned Eino v0.9.13 `adk.ChatModelAgentContext.Instruction`,
`ChatModelAgentMiddleware.BeforeAgent`, and `schema.SystemMessage`, plus the
existing `promptMiddleware.BeforeAgent` and `literalGenModelInput`. They already
install the committed instruction and generate the actual model input. No new
agent loop, middleware, resolver, store, or direct Eino import is needed.

Release packaging is outside this local development repair; no external push
or installed-product release is requested.
