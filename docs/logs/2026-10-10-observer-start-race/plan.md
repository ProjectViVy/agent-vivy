# Observer recovery startup race

Actual fresh-process recall race diagnostics exposed Start reading the pending recovery map concurrently with worker removal and OnRunEvent. Capture the RED with 200 repeated starts and concurrent notifications; protect the existing startup predicate with the existing pending-map mutex. Keep Journal replay, stable event IDs and durable cursors unchanged.
