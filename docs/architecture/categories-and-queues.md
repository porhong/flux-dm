# Categories and queues

Milestone 6 adds persisted organization policy without coupling it to the transfer engine.

## Category matching

Extensions are normalized to lowercase without a leading dot, de-duplicated, and sorted. A filename matches the highest-priority category; equal priority is resolved by stable category ID order. The result is therefore independent of SQLite row order. A matched category can supply the destination when an API caller leaves it blank.

## Queue dispatch

Pending jobs are selected by download priority plus queue priority. A bounded pool of 16 dispatch workers only admits jobs while their queue has capacity. Sequential queues have capacity one. The default unassigned queue has capacity three. Queue policies also cap segment connections. Each configured queue speed limit uses one shared engine limiter, so parallel downloads consume the queue budget collectively rather than receiving the full limit independently. An explicit per-download limit remains an additional cap for that transfer.

Assignments, priority, and insertion position are stored on each download. Category rules and queue policies use their own migrated SQLite tables, so restarting FluxDM preserves both policy and ordering. The Downloads toolbar can bulk-assign the current selection.

Stopped queues reject new starts explicitly. Running downloads are not killed when a policy is edited; the new policy applies to later admissions, avoiding implicit data-loss behavior.
