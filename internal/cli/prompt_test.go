package cli

import (
	"encoding/json"
	"testing"
)

func TestPromptCommandReturnsAgentPrompt(t *testing.T) {
	stdout, stderr, err := executeForTest("prompt")
	if err != nil {
		t.Fatalf("prompt failed: %v stderr=%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "agent_prompt" || got["tool"] != "aget" || got["audience"] != "llm_agent" {
		t.Fatalf("unexpected prompt payload: %#v", got)
	}
	if got["prompt"] == "" {
		t.Fatalf("prompt missing: %#v", got)
	}
}

func TestAgentInstructionsAliasMatchesPrompt(t *testing.T) {
	promptStdout, _, promptErr := executeForTest("prompt")
	if promptErr != nil {
		t.Fatal(promptErr)
	}
	aliasStdout, stderr, aliasErr := executeForTest("agent-instructions")
	if aliasErr != nil {
		t.Fatalf("agent-instructions failed: %v stderr=%s", aliasErr, stderr)
	}
	if aliasStdout != promptStdout {
		t.Fatalf("alias payload differs\nprompt=%s\nalias=%s", promptStdout, aliasStdout)
	}
}
