# P4.1a acceptance

## Engineering acceptance

Accepted for local Linux engineering scope. Injected logger selection,
profile-scoped host logging, conditional process-default restoration,
failed-startup cleanup, host-slot release, and permanent sink closure have
regression evidence. App tests and race-enabled logging/host suites pass.

## Product acceptance

Pending aggregate `just ci`, a built sealed product, and native Windows profile
open/close/remove acceptance in P7. The local gateway-less tests do not claim a
frontend build or installed-product acceptance.
