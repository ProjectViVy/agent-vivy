# QQ reconnect test observes protocol completion

`TestRedialResumeAndGiveUp` previously waited for the mock factory to publish
its second client, then immediately asserted that Connect and Resume had run.
Factory publication happens before the supervisor receives the client. Under
concurrent CI load the test observed a legitimate intermediate state and failed.

The test now waits on the existing synchronized fake counters until the second
attempt has connected, resumed, and entered Listening before checking carried
session state or dropping that attempt. The terminal cannot-identify case waits
for the existing supervisor `done` signal instead of sleeping 300 milliseconds.
Its exact attempt count and Stop idempotence assertions remain active.

This is a fixture-boundary correction only. Production QQ transport, protocol,
settings, grants, reconnect behavior, deadlines and public interfaces are unchanged.
The parent owns final QQ source pin/conformance artifact refresh and integrated CI.
No credentials, live QQ network, release artifact or deployment is involved.
