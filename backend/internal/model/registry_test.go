package model

import (
	"strings"
	"testing"
)

func withCatalogOverlay(t *testing.T, raw string) {
	t.Helper()
	if _, err := SetCatalogOverlayJSON(raw); err != nil {
		t.Fatalf("SetCatalogOverlayJSON(%q): %v", raw, err)
	}
	t.Cleanup(ResetCatalogOverlay)
}

// TestLookup_UnknownModelFallsBackToBillablePrice 回归测试：未注册模型必须走兜底定价，
// 绝不能返回 0 价（那会导致免费流量 / 计费全 0）。
func TestLookup_UnknownModelFallsBackToBillablePrice(t *testing.T) {
	spec := Lookup("some-brand-new-unknown-model-xyz")
	if spec.InputPrice <= 0 || spec.OutputPrice <= 0 {
		t.Fatalf("未知模型必须有兜底价格；InputPrice=%v OutputPrice=%v", spec.InputPrice, spec.OutputPrice)
	}
}

// TestLookup_ByKeyword 按关键词推断到对应系列。
func TestLookup_ByKeyword(t *testing.T) {
	cases := []struct {
		name      string
		modelID   string
		wantMatch string // 期望命中的注册键（按 InputPrice 识别）
	}{
		{"未知 codex 系列 → gpt-5.4", "gpt-5.9-codex-preview", "gpt-5.4"},
		{"未知 image 系列 → gpt-image-1.5", "gpt-image-3", "gpt-image-1.5"},
		{"未知 mini 系列 → gpt-5.4-mini", "gpt-5.9-mini", "gpt-5.4-mini"},
		{"未知 nano 系列 → gpt-5.4-mini", "gpt-5.9-nano", "gpt-5.4-mini"},
		{"未知 gpt-5 系列 → gpt-5.4", "gpt-5.9", "gpt-5.4"},
		{"o1 推理模型 → gpt-5.4", "o1-preview", "gpt-5.4"},
		{"o3 推理模型 → gpt-5.4", "o3-mini", "gpt-5.4-mini"}, // "mini" 优先
		{"gpt-4 系列 → gpt-5.4（偏保守）", "gpt-4o", "gpt-5.4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Lookup(tc.modelID)
			want := registry[tc.wantMatch]
			if got.InputPrice != want.InputPrice || got.OutputPrice != want.OutputPrice {
				t.Errorf("Lookup(%q): got InputPrice=%v OutputPrice=%v, want match %q (In=%v Out=%v)",
					tc.modelID, got.InputPrice, got.OutputPrice,
					tc.wantMatch, want.InputPrice, want.OutputPrice)
			}
		})
	}
}

// TestLookup_KnownModelUnchanged 已注册模型不受关键词兜底影响。
func TestLookup_KnownModelUnchanged(t *testing.T) {
	ResetCatalogOverlay()
	t.Run("gpt-5.5", func(t *testing.T) {
		spec := Lookup("gpt-5.5")
		if spec.InputPrice != 5.0 || spec.OutputPrice != 30.0 || spec.CachedPrice != 0.5 {
			t.Errorf("gpt-5.5 定价变化: In=%v Out=%v Cached=%v", spec.InputPrice, spec.OutputPrice, spec.CachedPrice)
		}
		if spec.ContextWindow != 400000 {
			t.Errorf("gpt-5.5 ContextWindow = %v, want 400000", spec.ContextWindow)
		}
		if spec.InputPricePriority != 12.5 || spec.OutputPricePriority != 75.0 || spec.CachedPricePriority != 1.25 {
			t.Errorf("gpt-5.5 priority 定价变化: In=%v Out=%v Cached=%v", spec.InputPricePriority, spec.OutputPricePriority, spec.CachedPricePriority)
		}
		if spec.InputPriceFast != 0 || spec.OutputPriceFast != 0 || spec.CachedPriceFast != 0 {
			t.Errorf("gpt-5.5 不应配置 fast 定价: In=%v Out=%v Cached=%v", spec.InputPriceFast, spec.OutputPriceFast, spec.CachedPriceFast)
		}
	})

	t.Run("gpt-5.4", func(t *testing.T) {
		spec := Lookup("gpt-5.4")
		if spec.InputPrice != 2.5 || spec.OutputPrice != 15.0 {
			t.Errorf("gpt-5.4 定价变化: In=%v Out=%v", spec.InputPrice, spec.OutputPrice)
		}
	})
}

func TestCatalogOverlay_OverrideExistingPrice(t *testing.T) {
	withCatalogOverlay(t, `[
	  {"id":"gpt-5.4","pricing":{"input":2,"cached_input":0.2,"output":10}}
	]`)

	spec := Lookup("gpt-5.4")
	if spec.InputPrice != 2 || spec.CachedPrice != 0.2 || spec.OutputPrice != 10 {
		t.Fatalf("gpt-5.4 overlay pricing = %+v, want input 2 cached 0.2 output 10", spec)
	}
	if spec.ContextWindow != registry["gpt-5.4"].ContextWindow {
		t.Fatalf("未覆盖字段不应改变, ContextWindow = %d", spec.ContextWindow)
	}
}

func TestCatalogOverlay_PartialPricingKeepsSafeDefaults(t *testing.T) {
	withCatalogOverlay(t, `[
	  {"id":"gpt-5.5","pricing":{"input":4,"cached_input":0,"output":0}}
	]`)

	spec := Lookup("gpt-5.5")
	base := registry["gpt-5.5"]
	if spec.InputPrice != 4 {
		t.Fatalf("显式 input 价格未覆盖: %+v", spec)
	}
	if spec.CachedPrice != base.CachedPrice || spec.OutputPrice != base.OutputPrice {
		t.Fatalf("缺省/0 价格不应覆盖成免费价: got %+v want cached %.2f output %.2f", spec, base.CachedPrice, base.OutputPrice)
	}
}

func TestCatalogOverlay_AddNewModel(t *testing.T) {
	withCatalogOverlay(t, `[
	  {"id":"gpt-6-preview","name":"GPT 6 Preview","context_window":500000,"max_output_tokens":128000,
	   "pricing":{"input":8,"cached_input":0.8,"output":40}}
	]`)

	if !IsKnown("gpt-6-preview") {
		t.Fatal("新增模型应被 IsKnown 识别")
	}
	spec := Lookup("gpt-6-preview")
	if spec.Name != "GPT 6 Preview" || spec.ContextWindow != 500000 ||
		spec.InputPrice != 8 || spec.CachedPrice != 0.8 || spec.OutputPrice != 40 {
		t.Fatalf("新增模型 spec = %+v", spec)
	}
	found := false
	for _, m := range AllModels() {
		if m.ID == "gpt-6-preview" {
			found = true
			if m.Name != "GPT 6 Preview" || m.ContextWindow != 500000 {
				t.Fatalf("AllModels 新增模型元信息错误: %+v", m)
			}
		}
	}
	if !found {
		t.Fatal("AllModels 应包含新增模型")
	}
}

func TestCatalogOverlay_DisabledModelHiddenButBillable(t *testing.T) {
	withCatalogOverlay(t, `[{"id":"gpt-5.4-mini","enabled":false}]`)

	if !IsKnown("gpt-5.4-mini") {
		t.Fatal("隐藏模型仍应保留在 registry 中用于兼容和计费")
	}
	if spec := Lookup("gpt-5.4-mini"); spec != registry["gpt-5.4-mini"] {
		t.Fatalf("隐藏模型计费规格不应改变: %+v", spec)
	}
	for _, m := range AllModels() {
		if m.ID == "gpt-5.4-mini" {
			t.Fatal("隐藏模型不应出现在 AllModels")
		}
	}
}

func TestCatalogOverlay_MalformedJSONDoesNotReplaceSnapshot(t *testing.T) {
	withCatalogOverlay(t, `[{"id":"gpt-5.4","pricing":{"input":2,"output":10}}]`)
	before := Lookup("gpt-5.4")
	if _, err := SetCatalogOverlayJSON(`{{{`); err == nil {
		t.Fatal("非法 JSON 应返回 error")
	}
	after := Lookup("gpt-5.4")
	if before != after {
		t.Fatalf("解析失败不应替换旧快照: before=%+v after=%+v", before, after)
	}
}

func TestCatalogOverlay_ImageModelCanBeAddedAndHidden(t *testing.T) {
	withCatalogOverlay(t, `[
	  {"id":"new-visual-model","name":"New Visual","image_only":true,
	   "pricing":{"input":6,"cached_input":0.6,"output":30}}
	]`)

	if !IsImageOnly("new-visual-model") {
		t.Fatal("image_only=true 的新增模型应被识别为纯图像模型")
	}
	var caps string
	for _, m := range AllModels() {
		if m.ID == "new-visual-model" {
			caps = strings.Join(m.Capabilities, ",")
		}
	}
	if caps != "image_generation" {
		t.Fatalf("新增图像模型 capabilities = %q, want image_generation", caps)
	}
}
