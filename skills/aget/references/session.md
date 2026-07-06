# aget session — session management

## Lifecycle

1. Open with `aget open URL -n NAME` and keep the returned `sid`.
2. Use page commands with `-s SID`.
3. Close with `aget session close -s SID` when finished.

```bash
aget session close -s SID
```

Closing ends the managed browser session. If you forget, inspect and clean up:

```bash
aget session list
aget session gc
```

## Profiles and concurrent sessions

A session opened with `--profile NAME` uses a persistent profile directory. One profile cannot be used by two active sessions at once; close the other session or use a different profile if you see `profile_in_use`.

## Batch execution

Use `aget batch -s SID --stdin` for known linear page workflows after probing. Batch stops at the first failure and has no branching.

```bash
printf '[
  {"cmd":"snapshot"},
  {"cmd":"click","ref":"@e1"},
  {"cmd":"wait","text":"Ready"},
  {"cmd":"get","kind":"url"}
]' | aget batch -s SID --stdin
```

For state-dependent flows, run commands one at a time and verify between actions.

## Inspect dashboard

```bash
aget inspect
aget inspect --port 9090
```

The dashboard shows active sessions and snapshots locally. Do not expose it publicly.
