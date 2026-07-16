# Local diagnostics and crash logs

FluxDM writes structured local diagnostics to `%APPDATA%\FluxDM\fluxdm.log`. Startup, recovery, scheduler, notification, and bridge failures use sanitized error categories; URLs, headers, cookies, tokens, credentials, and data-directory paths are not logged.

An unhandled panic on the application main path records its Go type and stack in the same local log before shutdown. Production builds use `-trimpath`, so build-machine source paths are removed. FluxDM does not upload logs or crash data. Users must inspect and explicitly share a log when requesting support.

The Settings **Clear private data** action truncates the log together with encrypted credentials and terminal histories without touching downloaded files. Release support should request the smallest relevant excerpt and treat every submitted log as potentially sensitive despite redaction.
