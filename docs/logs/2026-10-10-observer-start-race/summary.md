# Summary

Observer startup reads its pending recovery predicate under the same mutex as notifications and delivery removal. The original recovery wake still reaches the worker. No new queue, delivery owner or cursor reset.
