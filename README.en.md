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

To open with cookies (Netscape file or inline):

```bash
aget open https://example.com -n example --cookies cookies.txt
aget open https://example.com -n example --cookies "session=abc; token=xyz"
```

## Profiles

A profile is a named Chromium user-data directory with persistent cookies, localStorage, and session data. Create a profile once and reuse it across sessions to keep logged-in state.

```bash
# Create a profile with cookies (browser launches, injects cookies, then closes)
aget profile create ozon --cookies ozon-cookies.txt

# Create an empty profile (login manually via --headful)
aget profile create samokat

# Inspect
aget profile list
aget profile show ozon

# Open a page with a profile (cookies already inside)
aget open https://ozon.ru --profile ozon

# Delete a profile and all its data
aget profile delete ozon
```

A profile cannot be used by two sessions simultaneously — the second attempt returns an error.

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

### Input & Interaction

```bash
# Click by CSS selector
aget page click -s SID --selector "button[type=submit]"

# Type text character by character
aget page type -s SID --selector "input[name=q]" --text "agent browser workflow"
aget page type -s SID --ref @i1 --text TEXT

# Clear and fill
aget page fill -s SID --ref @i1 --text TEXT

# Press a key
aget page press -s SID --key Enter
```

### Dropdowns, Checkboxes & Radios

```bash
# Select an option in a <select> element
aget page select -s SID --ref @i1 --value VALUE
aget page select -s SID --selector "select[name=direction]" --value "Backend"

# Checkboxes and radios (idempotent: clicks only if state differs)
aget page check -s SID --ref @i1
aget page uncheck -s SID --ref @i1
```

### State Verification

```bash
aget page is -s SID --ref @i1 visible
aget page is -s SID --ref @i1 checked
aget page is -s SID --ref @i1 enabled
aget page is -s SID --ref @e1 focused
```

### Hover, Focus, File Upload, Dialogs

```bash
aget page hover -s SID --ref @e1
aget page focus -s SID --ref @i1
aget page upload -s SID --ref @i1 --file /path/to/resume.pdf
aget page dialog-accept -s SID
aget page dialog-accept -s SID --text "response"
aget page dialog-dismiss -s SID
```

### Universal Fallback

```bash
aget page js -s SID --expr "document.querySelector('input[name=x]').click()"
```

### Wait, read & scroll:

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

## Agent CLI Examples

Basic agent workflow:

```bash
aget open https://example.com -n research
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page wait -s SID --text "Done"
aget page read -s SID --limit 80
aget session close -s SID
```

Fill a form with snapshot refs:

```bash
aget page snapshot -s SID
aget page fill -s SID --ref @i1 --text "agent@example.com"
aget page select -s SID --ref @i2 --value "Backend"
aget page check -s SID --ref @i3
aget page is -s SID --ref @i3 checked
aget page press -s SID --key Enter
aget page get -s SID url
```

Work with a profile (cookies persist across sessions):

```bash
aget profile create mysite --cookies cookies.txt
aget open https://mysite.com --profile mysite
aget page snapshot -s SID
# ... actions on an authenticated page ...
aget session close -s SID
```

Run a multi-step workflow with one command:

```bash
printf '[{"cmd":"snapshot"},{"cmd":"fill","ref":"@i1","text":"agent@example.com"},{"cmd":"press","key":"Enter"},{"cmd":"wait","text":"Ready"},{"cmd":"get","kind":"url"}]' | aget batch -s SID --stdin
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
aget page select -s SID --ref @i1 --value VALUE
aget page check -s SID --ref @i1

Verify element state:
aget page is -s SID --ref @i1 visible
aget page is -s SID --ref @i1 checked

If visual state, layout, canvas, captcha, or a text-poor page matters, capture a screenshot:
aget page screenshot -s SID --path ./page.png

When refs are unavailable, use CSS selectors:
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT

To preserve authentication across sessions, use profiles:
aget profile create mysite --cookies cookies.txt
aget open URL --profile mysite

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
Codex: Use `aget` for browser tasks. Start with `aget open URL -n NAME`, keep the returned sid, then use `aget page snapshot -s SID` and refs for `click/fill/select/check`; verify with `aget page is`; use `aget page read` or `aget page get` for text and `aget page screenshot` for visual state. For authenticated sessions, create a profile: `aget profile create NAME --cookies FILE` then `aget open URL --profile NAME`. Close the session with `aget session close -s SID`.
```

```text
Claude Code: Before browser work, install/run `aget`. For each page, use the returned sid and JSON `next_commands`; start with `aget page snapshot`, act with refs (`fill/select/check/is`), use `page wait/get/scroll/batch` for multi-step workflows, and take a screenshot when layout matters or text reading is insufficient. For persistent cookies, use `aget profile create` and `open --profile`.
```

```text
OpenCode: Use `aget open`, then `aget page snapshot/read/click/fill/select/check/is/wait/get/scroll/screenshot` with the returned sid. Do not mix sid values across browser sessions, run `aget doctor` for browser problems, and always close the session when finished.
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
