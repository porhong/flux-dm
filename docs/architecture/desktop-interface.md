# Desktop interface architecture

## Scalable history

The downloads screen keeps the complete low-frequency history array but renders only the visible fixed-height window plus five overscan rows on each side. A 10,000-item automated test verifies that fewer than 50 semantic rows enter the DOM and that searching can surface an item outside the initial window. Search and state filters are memoized projections and never copy progress into the history array.

Each download row is memoized. High-frequency progress is held in one external signal per download and consumed with `useSyncExternalStore`, so a progress event rerenders only its matching visible row. Backend progress events remain limited to four per second per active download. Download metadata changes occur at the slower transactional checkpoint/state cadence.

## Daily-use commands

The header provides filename/URL search and state filtering. Selection works per row or across the visible filtered result, with bounded bulk Pause, Resume, and Cancel commands. Double-click or Enter opens properties. Space toggles selection, `P` invokes the applicable Pause/Resume/Retry/Restart command, Delete cancels an eligible transfer, Ctrl+A selects the filtered result, and Ctrl+N opens Add Download. Every icon-only control has an accessible name.

Row action menus provide properties and cancellation without expanding every row. Empty history, empty filter results, backend load failure, action failure, and unavailable-backend states have distinct UI output.

## Windows shell integration

Closing the main window hides it instead of terminating transfers. A bounded system-tray controller exposes Show FluxDM, Add download, and Exit. The explicit Exit command sets a termination guard before asking Wails to quit, while ordinary close continues background transfers. Tray Add shows the window and emits a frontend event that opens the Add Download dialog.

Completed transfers publish a native Windows toast through the registered FluxDM application identity. Toast bodies contain only the sanitized filename; source URLs, signed queries, headers, and destination paths are not included.
