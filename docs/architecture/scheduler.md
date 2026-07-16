# Scheduler

Schedules are local SQLite records evaluated every 30 seconds in the user's current time zone. A schedule selects one or more weekdays, an `HH:MM` time, an action, and a missed-run policy. Selecting every day is a daily schedule; selecting a subset produces a weekly schedule.

Before executing an occurrence, FluxDM inserts a history row with a unique `(schedule_id, run_key)`. Only the process that successfully inserts that claim may run the action. A crash or restart therefore cannot repeat an already-claimed occurrence. The history row is finalized as completed or failed after the action.

`skip` only admits an occurrence during its first minute. `run_once` admits the latest missed occurrence later that same eligible day. The durable claim still prevents repeated catch-up attempts.

Supported actions start or stop a queue, apply the global speed profile, or retry failed downloads. Optional Exit, Sleep, Hibernate, and Shutdown actions require an explicit acknowledgement in the UI. For queue-start and retry actions, the post-action waits until the affected downloads are no longer queued or active. Windows power requests use operating-system APIs and never interpolate user input into a command shell.
