# aget page snapshot — element discovery

Snapshot returns interactive elements as refs that agents can use without parsing HTML.

## Basic snapshot

```bash
aget page snapshot -s SID
```

Response elements include `ref` (`@e1`, `@i1`), `kind`, `text`, `selector`, `visible`, and `enabled`.

## Probe-first checklist

Before writing a multi-step workflow, probe the page:

```bash
aget open URL -n NAME
aget page snapshot -s SID
aget page read -s SID --limit 80
aget page find -s SID --role button
aget page find -s SID --role textbox
aget page is -s SID --ref @e1 visible
aget page is -s SID --ref @i1 enabled
```

For modals or dynamic states, open the state, wait for it, then snapshot again:

```bash
aget page click -s SID --ref @e1
aget page wait -s SID --text "Confirm"
aget page snapshot -s SID
```

Answer these before automating: what state is the page in, what triggers the next state, how completion is detected, what errors can appear, and whether banners/modals occlude the target.

## Snapshot with diff

After an action, show only what changed:

```bash
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page snapshot -s SID --diff
```

Use diffs to save tokens and verify that the expected state changed.

## Ref types

- `@e1`, `@e2`, ... — interactive elements such as buttons, links, role buttons, and tabindex targets.
- `@i1`, `@i2`, ... — inputs such as text fields, textareas, selects, checkboxes, radios, and file inputs.

## Ref lifecycle

Refs are generated per snapshot. They are stable for immediate follow-up actions but do not survive navigation or heavy DOM rerenders. Run snapshot again after page changes to get fresh refs.

## Errors

- `ref_not_found` — ref is stale or invalid; run `aget page snapshot -s SID` again.
