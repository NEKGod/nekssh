package main

import (
	"strings"
	"testing"
)

func TestAgentSkillTellsTheControlPort(t *testing.T) {
	if !strings.Contains(agentSkillMarkdown, "http://127.0.0.1:18765") || !strings.Contains(agentSkillMarkdown, "/v1/run") || !strings.Contains(agentSkillMarkdown, "name: run-on-nekssh-client") {
		t.Fatal("skill is missing the control port instructions")
	}
}
