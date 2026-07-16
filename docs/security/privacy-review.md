# Privacy review

FluxDM is local-first. Download history, schedules, profiles, logs, and the browser bridge file remain in the current user's FluxDM configuration directory. There is no telemetry, analytics, account sync, crash upload, or remote configuration service.

The browser extension requests cookies only when the user enables **Share browser cookies**. Those cookies are sent only to the native host/desktop bridge for the accepted download, encrypted with DPAPI at rest, and removed at completion/cancellation. Extension exclusions and toggles use browser sync storage according to the browser account's own sync policy.

UI profile lists return metadata and boolean secret-presence flags, never secret values. Logs do not record URLs, headers, cookies, tokens, or user data paths. Crash/release diagnostics are local and require explicit user action to share.

The **Clear private data** control deletes terminal download history, execution history, stored profiles/download cookies, and logs. It intentionally leaves downloaded files, active/paused transfers, categories, queues, and schedules untouched so a privacy reset cannot silently destroy user files or running work.
