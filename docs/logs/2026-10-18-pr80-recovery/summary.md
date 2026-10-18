# PR #80 recovery completion

Integrated latest main `7b42b2920f72de6f37de8b9e4ff4bf6a110a762f` into review
baseline `dfa755da63c45694528130aa6af00d4dde686f56`; merge `ad1aab43` preserves
main development-environment recipes, scripts, CI and work-control fixes. The
single conflict removed the obsolete open ToolsNode race row; its completed
record remains. Eino stays v0.9.21; no EinoExt pin changes.

Runtime candidate `e7255a7f46652541fe87716154d0e7cfa0207a95` persists a small
cleanup intent before changing pending interaction state. Startup drains it
before listing active Runs. Row cancellation, cancellation Journal event,
exactly one Run terminal, terminal status projection and Plan cancellation are
retryable across errors/restart. Errors retain causes and block successful
recovery until cleanup finishes. No DDL, checkpoint conversion or loosened
assertions/version checks. Existing lifecycle rows and Journal stay authoritative.

The policy now distinguishes upgrade to the fixed binary from historical
rollback, which requires fully verified cleanup before changing binaries.
Prior boot-only and version-string evidence is explicitly corrected.

No full AgenticModel/provider migration, #79 Recap/Subject change, deployment,
merge, compatibility rollback release or live provider-key check. B5 remains
P2 usage-projection work; B6/B7 remain authorized live-provider acceptance.
See verification.md for actual checks and remaining blockers. Release packaging
is not performed because this delivery updates the existing review PR only.
