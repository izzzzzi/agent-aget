# aget doctor — diagnostics

Run when install, browser resolution, permissions, or startup fails:

```bash
aget doctor
```

Returns JSON with per-check results:

```json
{
  "ok": false,
  "checks": [
    {"name": "state_dir", "ok": true, "message": "writable"},
    {"name": "browser", "ok": false, "message": "no browser found", "remediation": "run `aget browser install`, set AGET_BROWSER_PATH, or pass --browser-path to open"}
  ]
}
```

## Checks performed

1. `state_dir` — state directory exists and is writable.
2. `sessions_dir` — sessions directory is writable.
3. `artifacts_dir` — artifacts directory is writable.
4. `snapshots_dir` — snapshots directory is writable.
5. `browser` — managed CloakBrowser or configured browser resolves.
6. `state_permissions` — state files are not world-readable.
7. `encryption_key_unused` — warns if `AGET_ENCRYPTION_KEY` is set but unused.

## Browser management

```bash
aget browser status
aget browser install
aget browser path
```

Use these through `aget`; do not connect to an already-running browser to debug startup.
