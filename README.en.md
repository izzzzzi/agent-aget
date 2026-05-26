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

Start with a snapshot for actions. It returns refs like `@e1` and `@i1`:

```bash
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page fill -s SID --ref @i1 --text TEXT
```

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

Wait for state, get values, and scroll the page:

```bash
aget page wait -s SID --text "Ready"
aget page get -s SID text --ref @e1
aget page get -s SID url
aget page scroll -s SID --direction down --px 800
```

Capture a screenshot:

```bash
aget page screenshot -s SID
aget page screenshot -s SID --path ./page.png
```

Run multiple steps with one JSON batch command:

```bash
printf '[{"cmd":"click","ref":"@e1"},{"cmd":"wait","text":"Done"}]' | aget batch -s SID --stdin
```

Check install and browser startup diagnostics:

```bash
aget doctor
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

For actions, prefer snapshot refs before CSS selectors:
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page fill -s SID --ref @i1 --text TEXT

If visual state, layout, canvas, captcha, or a text-poor page matters, capture a screenshot:
aget page screenshot -s SID --path ./page.png

When refs are unavailable, use CSS selectors:
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT

For multi-step workflows, use:
aget page wait -s SID --text TEXT
aget page get -s SID text --ref REF
aget page scroll -s SID --direction down --px 800
aget batch -s SID --stdin

If install or browser startup fails:
aget doctor

Always close the session after the task:
aget session close -s SID

Do not repeat sensitive text from forms, cookies, tokens, or private pages. Continue the workflow with the returned sid and next_commands.
```

Minimal per-tool prompts:

```text
Codex: Use `aget` for browser tasks. Start with `aget open URL -n NAME`, keep the returned sid, then use `aget page snapshot -s SID` and refs for `click/fill`; use `aget page read` or `aget page get` for text and `aget page screenshot` for visual state. Close the session with `aget session close -s SID`.
```

```text
Claude Code: Before browser work, install/run `aget`. For each page, use the returned sid and JSON `next_commands`; start with `aget page snapshot`, act with refs, use `page wait/get/scroll/batch` for multi-step workflows, and take a screenshot when layout matters or text reading is insufficient.
```

```text
OpenCode: Use `aget open`, then `aget page snapshot/read/click/fill/wait/get/scroll/screenshot` with the returned sid. Do not mix sid values across browser sessions, run `aget doctor` for browser problems, and always close the session when finished.
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
