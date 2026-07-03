package gateway

import "testing"

func TestResolveAnthropicModelMapping_Fable5UsesOpusTarget(t *testing.T) {
	mapping := resolveAnthropicModelMapping("claude-fable-5")
	if mapping == nil {
		t.Fatal("mapping is nil")
	}
	if mapping.OpenAIModel != opusTargetModel {
		t.Fatalf("OpenAIModel = %q, want %q (opusTargetModel)", mapping.OpenAIModel, opusTargetModel)
	}
	if mapping.FallbackModel != opusFallbackModel {
		t.Fatalf("FallbackModel = %q, want %q (opusFallbackModel)", mapping.FallbackModel, opusFallbackModel)
	}
	if mapping.ReasoningEffort != "xhigh" {
		t.Fatalf("ReasoningEffort = %q, want xhigh", mapping.ReasoningEffort)
	}
}

// 覆盖未来可能出现的带日期快照别名（如 opus/sonnet 已有 claude-opus-4-5-20251101 这种
// 命名先例）；exact-match 表里只有裸 "claude-fable-5"，缺这条通配符会让快照变体漏判、
// 落到最后的通用 "claude-" 兜底（空 ReasoningEffort），而不是应有的 Opus 档待遇。
func TestResolveAnthropicModelMapping_Fable5DatedSnapshotUsesOpusTarget(t *testing.T) {
	mapping := resolveAnthropicModelMapping("claude-fable-5-20260601")
	if mapping == nil {
		t.Fatal("mapping is nil")
	}
	if mapping.OpenAIModel != opusTargetModel {
		t.Fatalf("OpenAIModel = %q, want %q (opusTargetModel)", mapping.OpenAIModel, opusTargetModel)
	}
	if mapping.ReasoningEffort != "xhigh" {
		t.Fatalf("ReasoningEffort = %q, want xhigh", mapping.ReasoningEffort)
	}
}

func TestResolveAnthropicModelMapping_Sonnet5UsesSonnetTarget(t *testing.T) {
	mapping := resolveAnthropicModelMapping("claude-sonnet-5")
	if mapping == nil {
		t.Fatal("mapping is nil")
	}
	if mapping.OpenAIModel != "gpt-5.5" {
		t.Fatalf("OpenAIModel = %q, want %q", mapping.OpenAIModel, "gpt-5.5")
	}
	if mapping.FallbackModel != "gpt-5.4" {
		t.Fatalf("FallbackModel = %q, want %q", mapping.FallbackModel, "gpt-5.4")
	}
	if mapping.ReasoningEffort != "high" {
		t.Fatalf("ReasoningEffort = %q, want high", mapping.ReasoningEffort)
	}
}

func TestResolveAnthropicModelMapping_UsesUpdatedDefaultClaudeTarget(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{name: "unknown model fallback", model: "claude-foo-9"},
		{name: "claude 3 wildcard fallback", model: "claude-3-7-legacy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping := resolveAnthropicModelMapping(tt.model)
			if mapping == nil {
				t.Fatal("mapping is nil")
			}
			if mapping.OpenAIModel != "gpt-5.5" {
				t.Fatalf("OpenAIModel = %q, want %q", mapping.OpenAIModel, "gpt-5.5")
			}
			if mapping.FallbackModel != "gpt-5.4" {
				t.Fatalf("FallbackModel = %q, want %q", mapping.FallbackModel, "gpt-5.4")
			}
		})
	}
}
