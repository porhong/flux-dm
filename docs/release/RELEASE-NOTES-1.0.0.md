# FluxDM 1.0.0

FluxDM 1.0.0 is the first complete Windows release. It includes adaptive segmented downloads with crash-safe resume, bandwidth controls, virtualized history, categories and bounded queues, daily/weekly schedules, Chrome/Edge integration, encrypted site profiles, native notifications, tray controls, and privacy/recovery hardening.

Security baseline: Go 1.26.5+, Windows current-user DPAPI, authenticated loopback browser bridge, strict native-message bounds, cross-authority credential stripping, explicit executable warnings, and no automatic file execution.

Known boundary: FluxDM does not bypass DRM, paywalls, browser security, authentication policy, or publisher signature requirements. Verify publisher hashes and Authenticode signatures for high-risk downloads.

The uninstaller terminates a running FluxDM/WebView2 process tree before Program Files cleanup and preserves local application data by default. Interactive uninstall can remove FluxDM settings, history, encrypted credentials, recovery backups, and logs; downloaded and unrecognized files are never selected for deletion.
