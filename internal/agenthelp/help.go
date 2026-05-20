package agenthelp

type HelpPayload struct {
	OK                 bool              `json:"ok"`
	Tool               string            `json:"tool"`
	Audience           string            `json:"audience"`
	Kind               string            `json:"kind"`
	CommandGroup       string            `json:"command_group,omitempty"`
	AgentPromptCommand string            `json:"agent_prompt_command"`
	Docs               []string          `json:"docs,omitempty"`
	Workflow           []string          `json:"workflow"`
	Commands           map[string]string `json:"commands"`
}

type PromptPayload struct {
	OK       bool   `json:"ok"`
	Tool     string `json:"tool"`
	Audience string `json:"audience"`
	Kind     string `json:"kind"`
	Prompt   string `json:"prompt"`
}

func RootHelp() HelpPayload {
	return HelpPayload{
		OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
		AgentPromptCommand: "aget prompt",
		Docs:               []string{"AGENT_INSTRUCTIONS.md", "README.md"},
		Workflow: []string{
			"Use browser status first if you need to verify the managed CloakBrowser backend",
			"Open a URL with aget open and keep the returned sid",
			"Continue with returned sid and next_commands",
			"Use page read for text extraction before deciding actions",
			"Use page screenshot when visual state matters",
			"Always close sessions with aget session close when finished",
		},
		Commands: map[string]string{
			"browser_status":  "aget browser status",
			"browser_install": "aget browser install",
			"open":            "aget open URL -n NAME",
			"page_read":       "aget page read -s SID --limit 80",
			"page_click":      "aget page click -s SID --selector CSS",
			"page_type":       "aget page type -s SID --selector CSS --text TEXT",
			"page_screenshot": "aget page screenshot -s SID --path ./page.png",
			"session_list":    "aget session list",
			"session_close":   "aget session close -s SID",
			"prompt":          "aget prompt",
		},
	}
}

func GroupHelp(name string) (HelpPayload, bool) {
	groups := map[string]HelpPayload{
		"page": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "page", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Use page read before click/type when possible",
				"Use CSS selectors for click/type",
				"Use screenshot when text output is insufficient",
			},
			Commands: map[string]string{
				"read":       "aget page read -s SID --limit 80",
				"click":      "aget page click -s SID --selector CSS",
				"type":       "aget page type -s SID --selector CSS --text TEXT",
				"screenshot": "aget page screenshot -s SID --path ./page.png",
			},
		},
		"browser": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "browser", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Use browser status to inspect the managed browser cache without network access",
				"Use browser install to download the pinned managed CloakBrowser stealth Chromium",
				"Use browser path to get the managed browser executable path",
			},
			Commands: map[string]string{
				"status":  "aget browser status",
				"install": "aget browser install",
				"path":    "aget browser path",
			},
		},
		"session": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "session", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Use returned sid values to continue browser workflows",
				"List sessions when you need to recover active sid values",
				"Always close sessions when finished",
			},
			Commands: map[string]string{
				"list":  "aget session list",
				"close": "aget session close -s SID",
				"gc":    "aget session gc",
			},
		},
		"open": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup: "open", AgentPromptCommand: "aget prompt",
			Workflow: []string{
				"Open a URL and keep the returned sid",
				"Follow next_commands from the response",
			},
			Commands: map[string]string{"open": "aget open URL -n NAME"},
		},
		"version": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup:       "version",
			AgentPromptCommand: "aget prompt",
			Workflow:           []string{"Use version for diagnostics only"},
			Commands:           map[string]string{"version": "aget version"},
		},
		"prompt": {
			OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_help",
			CommandGroup:       "prompt",
			AgentPromptCommand: "aget prompt",
			Workflow:           []string{"Load this prompt when an LLM agent needs usage instructions"},
			Commands: map[string]string{
				"prompt":             "aget prompt",
				"agent_instructions": "aget agent-instructions",
			},
		},
	}
	payload, ok := groups[name]
	return payload, ok
}

func Prompt() PromptPayload {
	return PromptPayload{
		OK: true, Tool: "aget", Audience: "llm_agent", Kind: "agent_prompt",
		Prompt: "You are using aget, a browser workflow CLI for LLM agents backed by managed CloakBrowser stealth Chromium when available. All operational commands return JSON. Use `aget browser status` to inspect the managed browser when needed. Start with `aget open URL`, save the returned `sid`, then use `aget page read -s SID --limit 80` for text, `aget page click -s SID --selector CSS` for clicks, `aget page type -s SID --selector CSS --text TEXT` for input, and `aget page screenshot -s SID --path ./page.png` when visual state matters. Continue with returned `next_commands` and always run `aget session close -s SID` when finished.",
	}
}
