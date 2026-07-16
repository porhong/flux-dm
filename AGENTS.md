# FluxDM Engineering Rules

## General

- Implement one milestone at a time.
- Do not combine unrelated refactors with feature work.
- Keep the download engine independent of Wails.
- Keep UI DTOs separate from database entities.
- Use `context.Context` for cancellation and timeouts.
- Never create unbounded goroutines or channels.
- Never buffer an entire download in memory.
- Never write secrets to logs.
- Never execute user-provided strings through a shell.
- Every state transition must use the centralized state machine.
- Every database change requires a migration.
- Every public backend method must validate its input.
- Prefer standard-library networking unless a dependency is justified.

## Required validation

Before marking a task complete, run:

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
npm run lint
npm run typecheck
npm run test
wails build
```

Run npm commands from `frontend/`.

## Testing

- Add unit tests for new domain behavior.
- Add integration tests for networking behavior.
- Include failure-path tests, not only successful cases.
- Use the local test server for deterministic download tests.
- Verify file hashes after segmented downloads.
- Check goroutine counts in repeated pause/resume tests.

## Frontend

- Use React and TypeScript strict mode.
- Use Tailwind CSS v4 and shadcn/ui.
- Avoid `any`.
- Do not store high-frequency progress for every download in one large object.
- Use memoized row components.
- Virtualize long lists.
- Batch Wails progress events.
- Keep feature code under `frontend/src/features`.

## Security

- Redact authorization, cookie, and signed-query values.
- Normalize and validate all destination paths.
- Reject unsupported URL schemes.
- Limit redirect counts and message sizes.
- Never automatically execute completed files.
