# Credentials, cookies, and proxies

Site profiles match an exact hostname or wildcard suffix. Exact/longer patterns win deterministically. A profile can provide HTTP Basic or Bearer authentication, a Cookie header, validated custom headers, and an HTTP/HTTPS proxy with separate credentials.

SQLite stores only profile metadata and a DPAPI ciphertext blob. Windows DPAPI uses the current-user scope with UI disabled; another Windows account cannot decrypt the data. Download-specific browser cookies use the same protection in a separate table and are deleted on completion or cancellation. The UI never reads secret values back—it exposes only `hasCredentials`, `hasCookies`, and header names—and provides an explicit **Clear secrets** action.

Header names must use the HTTP token grammar. Values reject CR/LF and have length/count limits. FluxDM reserves `Authorization`, `Cookie`, `Host`, `Content-Length`, `Range`, `If-Range`, `Proxy-Authorization`, connection, and transfer headers so custom fields cannot interfere with protocol safety. Authentication and cookie fields use their dedicated inputs.

Proxy URLs are limited to HTTP and HTTPS, must contain a hostname, and cannot embed credentials. Proxy usernames/passwords are kept in the encrypted payload and applied to a cloned Go HTTP transport. Protected-server and authenticated-proxy integration tests cover both probe and download request paths.

Structured logging redacts authorization, cookie, password, token, signature, API-key, and secret fields. Signed query values are redacted from messages. Request headers, cookie contents, and profile ciphertext are never logged.
