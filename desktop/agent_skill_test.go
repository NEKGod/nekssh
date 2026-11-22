package main

import (
	"strings"
	"testing"
)

func TestAgentSkillDiscoversRegisteredClients(t *testing.T) {
	if !strings.Contains(agentSkillMarkdown, ".nekssh/clients/*.json") || !strings.Contains(agentSkillMarkdown, "$CONTROL/v1/run") || !strings.Contains(agentSkillMarkdown, "name: run-on-nekssh-client") {
		t.Fatal("skill is missing client registry instructions")
	}
	if strings.Contains(agentSkillMarkdown, "127.0.0.1:18765") {
		t.Fatal("skill must not assume a fixed server control port")
	}
}
