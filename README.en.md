# agent-aget

`aget` is a browser workflow helper for LLM agents. The CLI starts a managed Chromium-compatible browser, stores local sessions, and returns machine-readable JSON.

## Install

```bash
npm i -g agent-aget
aget version
```

For local development:

```bash
go run ./cmd/aget version
```

## Quick Start

Open a page and keep the returned `sid`:

```bash
aget open https://example.com -n example
```

The browser runs headless by default. Use a visible window with:

```bash
aget open https://example.com -n example --headful
```

If the browser is not resolved automatically, pass its path:

```bash
AGET_BROWSER_PATH=/Applications/Chromium.app/Contents/MacOS/Chromium aget open https://example.com
aget open https://example.com --browser-path /path/to/chrome
```

## Page Commands

Read the current page:

```bash
aget page read -s SID
aget page read -s SID --limit 40
```

Click a CSS selector:

```bash
aget page click -s SID --selector "button[type=submit]"
```

Type text:

```bash
aget page type -s SID --selector "input[name=q]" --text "agent browser workflow"
```

Capture a screenshot:

```bash
aget page screenshot -s SID
aget page screenshot -s SID --path ./page.png
```

Close the session:

```bash
aget session close -s SID
```

## JSON Contract

Operational commands print one JSON object to stdout. Errors are printed to stderr:

```json
{"ok":false,"code":"invalid_args","message":"command required","details":{"hint":"run aget --help"}}
```

Successful responses include `ok: true`. `aget open` returns `sid`, browser details, the session record, and `next_commands` for follow-up actions.

## Environment

- `AGET_BROWSER_PATH` - path to a Chromium-compatible browser.
- `AGET_STATE_DIR` - local state directory for sessions, profiles, and artifacts.
- `AGENT_AGET_SKIP_DOWNLOAD=1` - skip the native binary download during npm `postinstall` and write a fake executable for smoke tests.

## License

MIT
