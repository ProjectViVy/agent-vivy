# Acceptance

1. While a Run is generating or suspended, call `background/recover` or refresh
   the Web face. Recovery reports a conflict and the original Run remains live.
2. Create an unused Session, select workspace B, and send relative attachment
   and context paths that exist only in B while the process project is A.
   The persisted user message contains B's inputs. Existing path escape and
   content validation still apply.
3. Open and reopen SQLite files containing literal `#`, `%23`, or spaces.
   On Linux also exercise `?mode=memory` in the filename. The exact file holds
   the saved Session and no URI parameter changes its identity.
4. Queue distinct requests with different thinking/mode and resolved inputs.
   A fresh Service reconstructs them in FIFO order with the same private bytes.
   Each follow-up creates a separate Run and receives its original options.
5. Interrupt the process after admission but before further model activity.
   The admitted Run's initial Journal record already owns the pending tail.
   Startup recovery admits still-pending requests once the Session is idle.
6. Inject a queue Journal write failure. RPC returns an error and does not
   acknowledge a new in-memory-only entry. Clear/remove/dequeue similarly retain
   the item on write failure; they remain usable after the old carrier seals.

Browser smoke used the split Vite face at `http://127.0.0.1:3015`, a local mock
provider, and an isolated home. Refresh showed the original active request and
both queued requests. After settlement, the page showed three distinct user
messages and three assistant replies, with no pending queue. The CODE face was
selected through public RPC for the synthetic inputs because this disposable
source assembly had no initialized companion persona.
