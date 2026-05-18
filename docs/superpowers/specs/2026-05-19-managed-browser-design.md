# Managed Browser Installation Design

## Summary

`aget` should work after npm installation without requiring the user to install or locate Chrome manually. The package will manage a pinned Chrome for Testing build in a user cache directory, while still allowing users to override the browser path with CloakBrowser or any Chromium-compatible binary.

The managed browser is best-effort during npm `postinstall`: native `aget` installation remains strict, but browser installation warnings do not fail package installation. Users can repair or inspect the managed browser explicitly with `aget browser ...` commands.

## Goals

- Install a Chromium-compatible browser automatically for common npm installs.
- Keep browser selection deterministic by pinning Chrome for Testing to the `agent-aget` release.
- Avoid modifying system Chrome or requiring package-manager privileges.
- Preserve explicit user control through `--browser-path` and `AGET_BROWSER_PATH`.
- Provide machine-readable JSON for status, install, path, and runtime errors.
- Keep CI and local smoke tests fast by allowing browser download to be skipped.

## Non-Goals

- Installing or updating system Google Chrome.
- Making CloakBrowser the default download source.
- Downloading a browser implicitly from `aget open`.
- Automatically deleting old cached browser versions in the MVP.
- Supporting non-Chromium browsers.

## Browser Source

The default managed browser will be Chrome for Testing / portable Chromium. CloakBrowser remains supported as an explicit override via `--browser-path` or `AGET_BROWSER_PATH`.

Chrome for Testing metadata and archives come from Google's official Chrome for Testing distribution endpoints. The repo will include a browser manifest that pins one exact browser version and platform-specific archive metadata.

## Cache Layout

Managed browser files live outside `AGET_STATE_DIR`.

Default cache root:

- Go runtime: `os.UserCacheDir()`
- npm installer: OS-specific user cache directory using Node APIs

Override:

- `AGET_BROWSER_CACHE_DIR`

Layout:

```text
<cache-root>/agent-aget/chrome-for-testing/<version>/<platform>/
```

The executable path is derived from the extracted archive layout for each platform.

## Manifest

A repo-level `browser-manifest.json` will be the shared contract between the Go CLI and npm installer.

It contains:

- pinned Chrome for Testing version
- platform keys
- archive names
- download URLs
- sha256 checksums
- relative executable paths after extraction

The manifest is included in the npm package and in release artifacts that need browser metadata.

## Install Flow

`scripts/install.js` keeps its current native `aget` download behavior. After the native binary is installed:

1. If `AGENT_AGET_SKIP_DOWNLOAD=1`, skip both native and browser download paths and keep current smoke-test behavior.
2. If `AGET_SKIP_BROWSER_DOWNLOAD=1`, skip only managed browser installation.
3. Read `browser-manifest.json`.
4. Resolve the current OS/arch platform entry.
5. Check whether the expected executable already exists and is executable.
6. If installed, exit successfully.
7. If missing, download the pinned Chrome for Testing archive to a temp directory.
8. Verify sha256.
9. Extract into a temp directory.
10. Move the extracted browser atomically into the cache layout.
11. Validate the executable.

If browser installation fails, `postinstall` prints a warning with `aget browser install` as the recovery command and still exits successfully. Native `aget` download failures still fail installation.

## Runtime Resolution

`aget open` resolves the browser in this order:

1. `--browser-path`
2. `AGET_BROWSER_PATH`
3. managed Chrome for Testing from cache
4. system browser candidates such as `chrome`, `chromium`, or `google-chrome`

If nothing is available, the JSON error uses:

- `code: "browser_not_found"`
- `details.recovery: "aget browser install"`
- `details.cache_dir`
- `details.env: "AGET_BROWSER_PATH"`

## CLI Commands

Add a `browser` command group.

### `aget browser status`

Does not access the network. Reports:

- `ok`
- expected browser version
- platform key
- cache directory
- expected executable path
- installed boolean
- executable boolean

### `aget browser install`

Strict command. Downloads and installs the pinned browser if needed.

Successful JSON:

- `ok`
- `version`
- `platform`
- `path`
- `cache_dir`
- `already_installed`

Failures return JSON errors with specific codes for unsupported platform, download failure, checksum mismatch, extraction failure, and executable validation failure.

### `aget browser path`

Does not access the network. Returns the managed browser executable path if installed. If missing, returns a JSON error with `details.recovery: "aget browser install"`.

## Update Model

The browser version is pinned to the `agent-aget` release.

When `agent-aget` is updated through npm, `postinstall` installs the newly pinned browser version if it is not already present. Old browser versions remain in cache for the MVP to avoid deleting files that could still be used by active sessions. A future `aget browser gc` command can clean unused versions.

## Error Handling

- npm native binary install remains strict.
- npm managed browser install is best-effort and warning-only.
- `aget browser install` is strict and returns machine-readable JSON errors.
- `aget browser status` and `aget browser path` never download data.
- Explicit browser overrides continue to return validation errors if the path is missing, a directory, or not executable.

## Testing

Unit coverage:

- platform mapping for npm and Go
- cache path layout
- manifest parsing
- executable path derivation
- resolver priority
- installed/missing status
- checksum mismatch
- browser installer failure behavior in `postinstall`

Smoke and release coverage:

- current npm smoke keeps `AGENT_AGET_SKIP_DOWNLOAD=1`
- add `AGET_SKIP_BROWSER_DOWNLOAD=1` for install paths that should skip only the browser
- optional gated integration test can download the real browser archive
- release contract verifies `browser-manifest.json` is included in npm packaging
- release contract verifies the `aget browser` command surface without requiring network access

## Open Decisions

There are no open decisions for the MVP. Future work may add `aget browser update` for latest stable and `aget browser gc` for old cached versions.
