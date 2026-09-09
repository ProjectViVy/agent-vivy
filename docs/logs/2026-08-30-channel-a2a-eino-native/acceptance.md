# Super Channel contract: Eino-native A2A clarification — acceptance

The contract answers the question "Can we still use Eino natively?":

| Sentence | Source | Expected |
|---|---|---|
| The five chat plugins do not block the ADK loop | §15.1, §21.7 | PASS |
| The native A2A protocol can reuse `eino-ext/a2a` models/transport | §15.1 table | PASS |
| `RegisterServerHandlers(adk.Agent)` is not a product path | §5.2, §15.1, §19 | PASS |
| The later execution path is ChannelHost → `Service.Run` | §15.1 layering, C9 | PASS |
| The default body must not import `eino-ext/a2a` | PLUGIN-SPEC prohibition | PASS |

There is no user-visible runtime behavior. Browser smoke testing is not applicable.
