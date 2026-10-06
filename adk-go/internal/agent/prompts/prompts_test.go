package prompts

import (
	"os"
	"strings"
	"testing"
)

// The LangGraph runtime must send exactly the same prompts.
func TestPromptsMatchLangGraphRuntime(t *testing.T) {
	source, err := os.ReadFile("../../../../langgraph-python/app/agent/prompts.py")
	if err != nil {
		t.Skipf("python prompts not available: %v", err)
	}
	for name, prompt := range map[string]string{"UnderstandRequest": UnderstandRequest, "EvaluateOptions": EvaluateOptions} {
		if !strings.Contains(string(source), prompt) {
			t.Errorf("%s differs from langgraph-python/app/agent/prompts.py", name)
		}
	}
}
