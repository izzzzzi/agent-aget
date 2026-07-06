# aget page actions — click, fill, type, select, check

Use refs from `aget page snapshot` or semantic locators from `aget page find` before CSS selectors. All actions stay inside `aget`; do not write browser automation scripts or direct protocol calls.

## Click

```bash
aget page click -s SID --ref @e1
aget page find -s SID --role button --name "Submit" --action click
```

Use `--selector` only when refs and semantic locators are not enough:

```bash
aget page click -s SID --selector "button[type=submit]"
```

### Force click

```bash
aget page click -s SID --ref @e1 --force
```

Use `--force` only after inspecting the state. It bypasses occlusion checks and sends the click through `aget` using real browser coordinates.

## Fill and type

`fill` clears first, then types. `type` keeps existing text and sends characters.

```bash
aget page fill -s SID --ref @i1 --text "user@example.com"
aget page type -s SID --ref @i1 --text " more text"
```

Responses include `text_len`, not the raw value. Do not try to reveal secrets by reading logs or retrying with debug commands.

## Select, check, and press

```bash
aget page select -s SID --ref @i1 --value option_value
aget page check -s SID --ref @i2
aget page uncheck -s SID --ref @i2
aget page press -s SID --key Enter
```

`check` and `uncheck` are idempotent: they act only when the current state differs.

## Hover, focus, upload, and dialogs

```bash
aget page hover -s SID --ref @e1
aget page focus -s SID --ref @i1
aget page upload -s SID --ref @i3 --file /path/to/file.pdf
aget page dialog-accept -s SID
aget page dialog-accept -s SID --text "response"
aget page dialog-dismiss -s SID
```

## Many similar elements

Do not write shell loops for browser actions. Probe first, then work through stable refs or `find --nth` values:

```bash
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page snapshot -s SID --diff
aget page click -s SID --ref @e2
```

```bash
aget page find -s SID --role button --name "Details" --nth 1 --action click
aget page find -s SID --role button --name "Details" --nth 2 --action click
```

If each click changes page state, verify with `read`, `snapshot --diff`, or `wait` before continuing.

## Batch sequences

`aget batch -s SID --stdin` is for known linear workflows after probing. It stops at the first failure and has no branching.

```bash
printf '[
  {"cmd":"snapshot"},
  {"cmd":"fill","ref":"@i1","text":"user@example.com"},
  {"cmd":"press","key":"Enter"},
  {"cmd":"wait","text":"Ready"},
  {"cmd":"get","kind":"url"}
]' | aget batch -s SID --stdin
```

Run steps one at a time when the next action depends on page state.

## JavaScript fallback

`aget page js` is a last-resort read/debug fallback. Do **not** use it for navigation, clicking, form filling, keyboard events, or cookie injection. Use real `aget` actions (`click`, `fill`, `type`, `select`, `check`, `press`, `upload`) with refs or semantic locators instead.

Safe examples are read-only diagnostics:

```bash
aget page js -s SID --expr "document.title"
aget page js -s SID --expr "location.href"
```

## Occlusion recovery

If a click returns `element_occluded`, inspect the page and either dismiss the blocker or deliberately use `--force`:

```bash
aget page snapshot -s SID
aget page click -s SID --ref @e4
aget page click -s SID --ref @e1 --force
```
