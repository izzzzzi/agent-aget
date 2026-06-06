# Agent Instructions

Use `aget` for browser work. Managed browser installs use CloakBrowser stealth Chromium by default.

Start with `aget open URL -n NAME`. Use the returned `sid` for all follow-up page commands.

Prefer `aget page snapshot -s SID` for actions. It returns refs like `@e1` (interactive elements: buttons, links) and `@i1` (inputs: text fields, selects, checkboxes, file inputs). Use refs before CSS selectors when possible:

```bash
aget page click -s SID --ref @e1
aget page fill -s SID --ref @i1 --text TEXT
aget page select -s SID --ref @i1 --value VALUE
aget page check -s SID --ref @i1
aget page uncheck -s SID --ref @i1
```

Use CSS selectors only when snapshot refs are unavailable:

```bash
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT
aget page select -s SID --selector CSS --value VALUE
```

Use these commands to inspect and advance browser workflows:

```bash
# Reading state
aget page read -s SID --limit 80
aget page get -s SID url
aget page get -s SID title
aget page get -s SID text --ref REF
aget page get -s SID value --ref REF

# Verifying state
aget page is -s SID --ref REF visible
aget page is -s SID --ref REF checked
aget page is -s SID --ref REF enabled
aget page is -s SID --ref REF focused

# Interaction
aget page click -s SID --ref REF
aget page fill -s SID --ref REF --text TEXT
aget page select -s SID --ref REF --value VALUE
aget page type -s SID --ref REF --text TEXT
aget page check -s SID --ref REF
aget page uncheck -s SID --ref REF
aget page press -s SID --key Enter
aget page hover -s SID --ref REF
aget page focus -s SID --ref REF

# Navigation
aget page wait -s SID --text TEXT
aget page wait -s SID --ref REF
aget page wait -s SID --load ready
aget page scroll -s SID --direction down --px 800

# File upload
aget page upload -s SID --ref REF --file /path/to/file.pdf

# Dialogs
aget page dialog-accept -s SID
aget page dialog-accept -s SID --text "response"
aget page dialog-dismiss -s SID

# Universal fallback
aget page js -s SID --expr "document.querySelector('input[name=x]').click()"

# Visual
aget page screenshot -s SID --path ./page.png

# Batch
aget batch -s SID --stdin

# Profiles (persistent cookies across sessions)
aget profile create ozon --cookies ozon-cookies.txt
aget profile create samokat --cookies samokat-cookies.txt
aget profile list
aget profile show ozon
aget open https://ozon.ru --profile ozon
aget open https://samokat.ru --profile samokat
aget profile delete ozon

# Device emulation (coherent viewport + user-agent + touch for stealth)
aget open https://m.site.ru --device mobile
aget open https://m.site.ru --device tablet
```

Run `aget doctor` when install or browser startup fails.

Always close sessions with `aget session close -s SID` when finished.

Do not paste secrets into examples or logs. Avoid echoing sensitive text from forms, cookies, tokens, or private pages. Put secrets into environment variables and pass them to commands only when needed.
