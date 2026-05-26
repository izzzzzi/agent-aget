# Agent Instructions

Use `aget` for browser work. Managed browser installs use CloakBrowser stealth Chromium by default.

Start with `aget open URL -n NAME`. Use the returned `sid` for all follow-up page commands.

Prefer `aget page snapshot -s SID` for actions. It returns refs like `@e1` and `@i1`. Use refs before CSS selectors when possible:

```bash
aget page click -s SID --ref @e1
aget page fill -s SID --ref @i1 --text TEXT
```

Use CSS selectors only when snapshot refs are unavailable:

```bash
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT
```

Use these commands to inspect and advance browser workflows:

```bash
aget page read -s SID --limit 80
aget page wait -s SID --text TEXT
aget page get -s SID text --ref REF
aget page get -s SID url
aget page scroll -s SID --direction down --px 800
aget page screenshot -s SID --path ./page.png
aget batch -s SID --stdin
```

Run `aget doctor` when install or browser startup fails.

Always close sessions with `aget session close -s SID` when finished.

Do not paste secrets into examples or logs. Avoid echoing sensitive text from forms, cookies, tokens, or private pages. Put secrets into environment variables and pass them to commands only when needed.
