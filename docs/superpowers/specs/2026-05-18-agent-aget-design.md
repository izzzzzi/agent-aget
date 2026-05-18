# agent_aget design

## Purpose

`agent_aget` is a separate browser automation CLI for LLM agents. It mirrors the operating model of `agent_ssh`: stable JSON responses, persistent sessions, compact reads, local registry metadata, and cross-platform binary releases with an npm installer wrapper.

The public binary name is `aget`. The local project directory remains `agent_aget` to match the existing `agent_ssh` workspace style.

## Scope

The first version focuses on browser sessions, not a general HTTP fetcher and not a full anti-detect profile manager.

Included in MVP:

- Start a headless browser session for a URL.
- Return a stable `sid` and `next_commands` JSON object after opening a session.
- Read page state in a compact agent-friendly format.
- Click elements by selector.
- Type text into elements by selector.
- Capture screenshots to local files.
- List, close, and garbage-collect local sessions.
- Store session registry data under `~/.aget/sessions`.
- Provide a `--headful` flag for debugging or manual login.

Deferred from MVP:

- Proxy management.
- Fingerprint/profile management.
- Download management.
- Multi-tab workflows beyond the active page.
- Full Playwright-compatible selector semantics.
- A high-level scraping framework.

## Architecture

`aget` is implemented in Go as a standalone CLI. It manages an external CloakBrowser-compatible Chromium binary and talks to it through the Chrome DevTools Protocol.

The CLI owns:

- Browser process lifecycle.
- Remote debugging port selection.
- Session registry metadata.
- JSON command responses.
- CDP command execution.
- Compact page extraction for agents.

CloakBrowser owns:

- The Chromium runtime.
- Browser behavior and rendering.
- Any CloakBrowser-specific runtime behavior.

The Go CLI does not require Python or Node at runtime.

## Commands

### `aget open`

Starts a browser process if needed, opens the requested URL, creates a local session, and returns JSON.

Example:

```bash
aget open https://example.com -n work
```

Response shape:

```json
{
  "ok": true,
  "sid": "f7a2b3c4",
  "session": "work",
  "browser": {
    "headless": true
  },
  "page": {
    "url": "https://example.com",
    "title": "Example Domain"
  },
  "next_commands": {
    "read": "aget page read -s f7a2b3c4 --limit 80",
    "click": "aget page click -s f7a2b3c4 --selector SELECTOR",
    "type": "aget page type -s f7a2b3c4 --selector SELECTOR --text TEXT",
    "screenshot": "aget page screenshot -s f7a2b3c4",
    "close": "aget session close -s f7a2b3c4"
  }
}
```

### `aget page read`

Reads the current active page in a compact format. It returns title, URL, visible text excerpt, and selected interactive elements such as links, buttons, inputs, and forms. Large output is paginated with `--limit` and later may support cursor-like offsets.

### `aget page click`

Clicks an element by selector. MVP selectors are intentionally narrow: CSS selectors first, with a later extension point for text selectors such as `text=Login`.

### `aget page type`

Types text into an input-like element by selector. Secrets should be passed through environment variables by callers when needed, not embedded in logs or command examples.

### `aget page screenshot`

Writes a screenshot under `~/.aget/artifacts` by default and returns the path in JSON.

### `aget session list|close|gc`

Manages local browser sessions. `close` terminates the browser process associated with a session. `gc` cleans up stale session metadata and orphaned processes that match trusted `aget` metadata.

## Defaults

- Browser mode is headless by default.
- `--headful` switches to a visible browser.
- Responses are JSON by default.
- Commands should avoid printing large page content unless explicitly requested.
- Local state is stored under `~/.aget`.

## Error Handling

All command failures return machine-readable JSON with:

- `ok: false`
- `code`
- `message`
- optional `details`
- optional `next_commands`

Expected error categories:

- CloakBrowser binary missing or unsupported.
- Browser process failed to start.
- CDP connection failed.
- Navigation timeout.
- Selector not found.
- Session not found or stale.
- Screenshot write failure.

## Testing

The project should follow the same practical testing style as `agent_ssh`:

- Unit tests for CLI argument parsing, registry behavior, JSON response shapes, and path handling.
- Unit tests for selector validation and page extraction formatting.
- Integration tests that can run against a local static test page using a browser binary when available.
- CI should allow browser-dependent tests to be skipped when the binary is unavailable, while keeping pure Go tests mandatory.

## Release Model

The release model should mirror `agent_ssh`:

- Go module in this repository.
- Cross-platform binaries for macOS, Linux, and Windows.
- npm package wrapper that installs the right binary.
- GitHub Releases as the binary distribution source.

## Open Decisions

- Exact CloakBrowser binary acquisition flow: bundled downloader, user-provided path, or both.
- Final public repository/module path.
- Whether the initial selector language includes `text=` or only CSS selectors.
- Whether one-shot aliases such as `aget get URL` and `aget screenshot URL` ship in v1 or after the session API is stable.
