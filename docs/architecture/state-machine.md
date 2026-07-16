# Download state machine

Milestone 2 centralizes normal and startup-recovery transitions in `internal/download`.

## States

```text
queued
probing
preparing
downloading
pausing
paused
retrying
completed
failed
cancelled
deleted
```

## Normal transitions

```text
queued -> probing -> preparing -> downloading
downloading -> completed
downloading -> pausing -> paused
paused -> preparing -> downloading
failed -> retrying -> preparing -> downloading
paused | failed | cancelled -> queued (explicit restart)
probing | preparing | downloading | pausing | retrying -> failed
queued | probing | preparing | downloading | pausing | paused | retrying -> cancelled
```

Startup recovery is deliberately separate from user-driven transitions:

```text
probing -> queued | failed
preparing | downloading | pausing | retrying -> queued | paused | failed
```

Implementation rules:

- A transition is an explicit domain operation, never a direct field assignment.
- Invalid transitions return a typed application error and leave persisted state unchanged.
- State and associated segment/checkpoint updates are committed transactionally.
- Recovery maps only interrupted transient states, after reconciling the temporary file. Remote validators are checked during the subsequent resume attempt.
- UI labels are projections of domain state and never control transition validity.

`paused` means no request is active and the temporary file has been closed. `failed` retains a safe partial file when retry is possible. A failure caused by changed validators, missing range support, or inconsistent local data requires an explicit restart from byte zero.
