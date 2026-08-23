package claude

import "testing"

func TestDefaultModels_ContainsClaudeOpus48(t *testing.T) {
	t.Parallel()

	// Opus 4.8 需要出现在默认目录中，供 /v1/models 与后台模型选择器复用。
	byID := make(map[string]Model, len(DefaultModels))
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}

	model, ok := byID["claude-opus-4-8"]
	if !ok {
		t.Fatal("expected claude-opus-4-8 to be exposed in DefaultModels")
	}
	if model.DisplayName != "Claude Opus 4.8" {
		t.Fatalf("unexpected display name: %q", model.DisplayName)
	}
	if model.CreatedAt != "2026-05-29T00:00:00Z" {
		t.Fatalf("unexpected created_at: %q", model.CreatedAt)
	}

	ids := DefaultModelIDs()
	for _, id := range ids {
		if id == "claude-opus-4-8" {
			return
		}
	}
	t.Fatal("expected claude-opus-4-8 to be exposed in DefaultModelIDs")
}
