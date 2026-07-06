# aget open — opening URLs

## Basic open

```bash
aget open https://example.com -n example
```

Returns JSON with `sid` and `next_commands`. Use the `sid` for all follow-up page commands and close it when finished.

## Headful mode

For visual debugging, login flows, or pages where seeing the browser matters:

```bash
aget open https://example.com -n example --headful
```

Do not connect to a user’s already-running browser; `aget open` starts the managed browser session.

## Device emulation

```bash
aget open https://m.example.com --device mobile
aget open https://m.example.com --device tablet
```

Device mode sets coherent viewport, user-agent, and touch behavior.

## Cookies

Use cookie files with `aget`; never inject cookies through raw browser protocols or scripts.

```bash
aget open https://example.com -n example --cookies cookies.txt
```

Inline cookie strings are accepted when necessary, but avoid putting secrets in shell history or logs:

```bash
aget open https://example.com -n example --cookies "session=abc; token=xyz"
```

## Profiles

Create a profile once with cookies, then reuse it:

```bash
aget profile create mysite --cookies cookies.txt
aget open https://mysite.com --profile mysite
```

Save auth state from a current `aget` session, including localStorage for SPAs:

```bash
aget profile save mysite -s SID
```

Profiles persist cookies, localStorage, and session data. One profile cannot be used by two sessions at the same time.

## Clean reads for a session

```bash
aget open https://example.com --clean
```

This enables `--clean` for all `aget page read` calls in the session.

## Browser resolution and diagnostics

`aget` uses managed CloakBrowser by default. If startup fails, inspect status and run diagnostics:

```bash
aget browser status
aget doctor
```
