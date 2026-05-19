# Agent Help and Prompt Design

## Summary

`aget` is a CLI for LLM agents, so `--help` should be an agent entrypoint instead of a disabled Cobra help path. The CLI will return compact JSON help that tells an agent what the tool is, where to get the full prompt, and which command flow to use. A separate prompt command will return a fuller agent instruction payload.

This keeps the existing machine-readable JSON contract while making the tool discoverable.

## Goals

- Make `aget --help` and command-group help useful to LLM agents.
- Preserve JSON output for help, success, and errors.
- Add a short full-tool prompt command that agents can load into context.
- Keep help concise and stable enough for automated parsing.
- Fix the current dead-end hint where errors say to run help, but help is disabled.

## Non-Goals

- Reintroducing standard Cobra human-readable help.
- Producing long manuals from `--help`.
- Adding interactive help, shell completion, or pager behavior.
- Changing operational command JSON shapes.

## Help Contract

`aget --help` returns a JSON object shaped as an agent entrypoint:

```json
{
  "ok": true,
  "tool": "aget",
  "audience": "llm_agent",
  "kind": "agent_help",
  "agent_prompt_command": "aget prompt",
  "docs": ["AGENT_INSTRUCTIONS.md", "README.md"],
  "workflow": [
    "Use browser status first if you need to verify the managed browser",
    "Open a URL with aget open and keep the returned sid",
    "Continue with returned sid and next_commands",
    "Use page read for text extraction before deciding actions",
    "Use page screenshot when visual state matters",
    "Always close sessions with aget session close when finished"
  ],
  "commands": {
    "browser_status": "aget browser status",
    "browser_install": "aget browser install",
    "open": "aget open URL -n NAME",
    "page_read": "aget page read -s SID --limit 80",
    "page_click": "aget page click -s SID --selector CSS",
    "page_type": "aget page type -s SID --selector CSS --text TEXT",
    "page_screenshot": "aget page screenshot -s SID --path ./page.png",
    "session_list": "aget session list",
    "session_close": "aget session close -s SID",
    "prompt": "aget prompt"
  }
}
```

`aget <group> --help` returns the same high-level fields, plus a scoped command group. For example `aget page --help`:

```json
{
  "ok": true,
  "tool": "aget",
  "audience": "llm_agent",
  "kind": "agent_help",
  "command_group": "page",
  "agent_prompt_command": "aget prompt",
  "workflow": [
    "Use page read before click/type when possible",
    "Use CSS selectors for click/type",
    "Use screenshot when text output is insufficient"
  ],
  "commands": {
    "read": "aget page read -s SID --limit 80",
    "click": "aget page click -s SID --selector CSS",
    "type": "aget page type -s SID --selector CSS --text TEXT",
    "screenshot": "aget page screenshot -s SID --path ./page.png"
  }
}
```

The first implementation will cover root help and the main command groups: `browser`, `page`, `session`, `open`, `version`, and `prompt`. Unknown command help remains an `invalid_args` JSON error.

## Prompt Contract

Add `aget prompt` and alias `aget agent-instructions`.

Both commands return JSON:

```json
{
  "ok": true,
  "tool": "aget",
  "audience": "llm_agent",
  "kind": "agent_prompt",
  "prompt": "You are using aget, a browser workflow CLI for LLM agents. All operational commands return JSON. Start with `aget open URL`, save `sid`, then use `aget page read -s SID`..."
}
```

The prompt should be short enough to load into an LLM context directly. It covers:

- what `aget` does
- JSON contract
- managed browser check/install
- open/read/click/type/screenshot/close flow
- using returned `sid` and `next_commands`
- cleanup expectations
- common recovery commands

## Error Hints

Invalid argument errors keep the JSON error shape, but the hint changes from a dead-end generic help instruction to an agent-oriented instruction:

```json
{
  "hint": "run `aget --help` for agent command map or `aget prompt` for full agent instructions"
}
```

Command-specific errors may keep their existing details, but new help hints should point to `aget prompt` when the agent needs usage context.

## Implementation Shape

Create a small internal help/prompt unit rather than spreading prompt literals through command files.

Suggested package:

- `internal/agenthelp`

Responsibilities:

- define help payload structs or maps
- return root and command-group help payloads
- return prompt payload
- keep text centralized and testable

CLI integration:

- Replace the disabled help flag behavior with a custom help flag that writes JSON help and exits successfully.
- Register `prompt` and `agent-instructions` commands.
- Keep standard Cobra completion disabled.
- Continue writing operational errors to stderr and successful help/prompt payloads to stdout.

## Testing

Unit tests should verify:

- `aget --help` exits successfully and returns JSON with `tool: "aget"`, `audience: "llm_agent"`, `agent_prompt_command: "aget prompt"`, and core command map entries.
- `aget page --help` exits successfully and returns scoped page commands.
- `aget browser --help` exits successfully and returns scoped browser commands.
- `aget prompt` returns `kind: "agent_prompt"` and includes the core workflow text.
- `aget agent-instructions` returns the same payload as `aget prompt`.
- invalid argument errors include the new non-dead-end hint.
- unknown command plus `--help` remains a JSON `invalid_args` error.

Release/package checks should ensure `AGENT_INSTRUCTIONS.md` stays in the npm package if help advertises it.

## Documentation

Update README files to mention:

- `aget --help` is JSON agent help, not standard Cobra help.
- `aget prompt` / `aget agent-instructions` prints the prompt an LLM agent should load.
- The CLI intentionally optimizes help for agent workflows.

## Open Decisions

There are no open decisions for the MVP. Future work can add `--human-help` or markdown output if human-facing CLI help becomes important.
