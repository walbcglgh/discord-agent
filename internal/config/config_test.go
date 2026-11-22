package config

import "testing"

func TestDefaultAndClone(t *testing.T) {
	t.Setenv("PANEL_PASSWORD", "test-password")
	c := Default()
	c.Panel.PasswordHash = "$2a$10$abcdefghijklmnopqrstuv"
	if err := validate(c); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
	c.LLM.Models = append(c.LLM.Models, ModelConfig{
		ID: "m_1", Name: "DeepSeek V4.1 Flash", Protocol: ProtocolOpenAIChat,
		APIKeyEnv: "KEY", ModelID: "deepseek-chat", Enabled: true,
	})
	clone, err := c.Clone()
	if err != nil {
		t.Fatal(err)
	}
	clone.LLM.Models[0].Name = "Other"
	if c.LLM.Models[0].Name != "DeepSeek V4.1 Flash" {
		t.Error("Clone must deep-copy model slice")
	}
}

func TestSecret(t *testing.T) {
	t.Setenv("TEST_KEY", "from-env")
	if got := Secret("TEST_KEY", ""); got != "from-env" {
		t.Errorf("env fallback failed: %q", got)
	}
	if got := Secret("TEST_KEY", "inline"); got != "inline" {
		t.Errorf("inline should win: %q", got)
	}
}
