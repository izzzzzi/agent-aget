# agent-aget

[![CI](https://github.com/izzzzzi/agent-aget/actions/workflows/ci.yml/badge.svg)](https://github.com/izzzzzi/agent-aget/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/izzzzzi/agent-aget)](https://github.com/izzzzzi/agent-aget/releases)
[![npm](https://img.shields.io/npm/v/agent-aget)](https://www.npmjs.com/package/agent-aget)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Language: English | [Русский](README.md)

`aget` is a browser workflow helper for LLM agents. The CLI starts managed [CloakBrowser](https://github.com/CloakHQ/CloakBrowser) stealth Chromium, stores local sessions, and returns machine-readable JSON.

## Install

```bash
npm i -g agent-aget
aget version
```

During `npm i -g agent-aget`, the package downloads the native `aget` binary and tries to install pinned CloakBrowser into the user cache. [Upstream CloakBrowser](https://github.com/CloakHQ/CloakBrowser) describes itself as: "Stealth Chromium that passes every bot detection test. Drop-in Playwright replacement with source-level fingerprint patches. 30/30 tests passed." If the network is unavailable, package installation continues; install the browser later with:

```bash
aget browser install
aget browser status
aget browser path
```

Browser resolution order:

1. `--browser-path`
2. `AGET_BROWSER_PATH`
3. managed CloakBrowser from cache
4. legacy managed Chrome for Testing from cache, if it was installed by earlier `aget` versions
5. system Chrome/Chromium

To skip managed browser installation:

```bash
AGET_SKIP_BROWSER_DOWNLOAD=1 npm i -g agent-aget
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
{"ok":false,"code":"invalid_args","message":"command required","details":{"hint":"run `aget --help` for agent command map or `aget prompt` for full agent instructions"}}
```

Successful responses include `ok: true`. `aget open` returns `sid`, browser details, the session record, and `next_commands` for follow-up actions.

## Agent Help

`aget --help` returns a JSON command map for an LLM agent, not standard Cobra help:

```bash
aget --help
aget page --help
```

For the full short instruction prompt, load:

```bash
aget prompt
aget agent-instructions
```

These commands keep the CLI JSON contract.

## Agent CLI Examples

Paste this instruction into Codex, Claude Code, OpenCode, or another terminal agent before browser work:

```text
Use `aget` for browser tasks.

First load the short instruction:
aget prompt

Open the target URL:
aget open URL -n NAME

Keep the returned sid. To understand the page, start with:
aget page read -s SID --limit 80

If visual state, layout, canvas, captcha, or a text-poor page matters, capture a screenshot:
aget page screenshot -s SID --path ./page.png

For clicks and typing, use CSS selectors:
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT

Always close the session after the task:
aget session close -s SID

Do not repeat sensitive data from forms, cookies, tokens, or private pages. Continue the workflow with the returned sid and next_commands.
```

Minimal per-tool prompts:

```text
Codex: Use `aget` for browser tasks. Start with `aget open URL -n NAME`, keep the returned sid, then read the page with `aget page read -s SID --limit 80`; use `aget page screenshot` for visual state. Close the session with `aget session close -s SID`.
```

```text
Claude Code: Before browser work, install/run `aget`. For each page, use the returned sid and JSON `next_commands`; read text first with `aget page read`, and take a screenshot when layout matters or text reading is insufficient.
```

```text
OpenCode: Use `aget open`, then `aget page read/click/type/screenshot` with the returned sid. Do not mix sid values across browser sessions, and always close the session when finished.
```

## Environment

- `AGET_BROWSER_PATH` - path to a Chromium-compatible browser.
- `AGET_BROWSER_CACHE_DIR` - cache directory for managed CloakBrowser.
- `AGET_STATE_DIR` - local state directory for sessions, profiles, and artifacts.
- `AGENT_AGET_SKIP_DOWNLOAD=1` - skip the native binary download during npm `postinstall` and write a fake executable for smoke tests.

## License

MIT

## Russian

See [README.md](README.md).
