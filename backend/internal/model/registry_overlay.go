package model

import (
	"encoding/json"
	"strings"
	"sync/atomic"
)

// CatalogStats 描述一次模型目录覆盖层应用后的有效规模。
type CatalogStats struct {
	RegistrySize int
	HiddenSize   int
}

type catalogOverlay struct {
	registry map[string]Spec
	hidden   map[string]bool
}

var overlayStore atomic.Pointer[catalogOverlay]

func activeRegistry() map[string]Spec {
	if ov := overlayStore.Load(); ov != nil {
		return ov.registry
	}
	return registry
}

func activeHiddenModels() map[string]bool {
	if ov := overlayStore.Load(); ov != nil {
		return ov.hidden
	}
	return nil
}

// ResetCatalogOverlay 清空运行时覆盖层，恢复纯内置注册表。
func ResetCatalogOverlay() {
	overlayStore.Store(nil)
}

// SetCatalogOverlayJSON 解析并应用后台配置的模型目录覆盖层。
//
// 空字符串表示空覆盖层。非法 JSON 返回错误且不修改当前快照；调用方可安全保留旧配置。
func SetCatalogOverlayJSON(raw string) (CatalogStats, error) {
	ov, err := parseCatalogOverlay(raw)
	if err != nil {
		return CatalogStats{}, err
	}
	overlayStore.Store(ov)
	return CatalogStats{RegistrySize: len(ov.registry), HiddenSize: len(ov.hidden)}, nil
}

type overlayPricing struct {
	Input               float64 `json:"input"`
	CachedInput         float64 `json:"cached_input"`
	Output              float64 `json:"output"`
	PriorityInput       float64 `json:"priority_input"`
	PriorityCachedInput float64 `json:"priority_cached_input"`
	PriorityOutput      float64 `json:"priority_output"`
	FlexInput           float64 `json:"flex_input"`
	FlexCachedInput     float64 `json:"flex_cached_input"`
	FlexOutput          float64 `json:"flex_output"`
}

type overlayLongContext struct {
	Threshold        int     `json:"threshold"`
	InputMultiplier  float64 `json:"input_multiplier"`
	CachedMultiplier float64 `json:"cached_multiplier"`
	OutputMultiplier float64 `json:"output_multiplier"`
}

type overlayEntry struct {
	ID            string              `json:"id"`
	Name          string              `json:"name,omitempty"`
	ContextWindow int                 `json:"context_window,omitempty"`
	MaxOutput     int                 `json:"max_output_tokens,omitempty"`
	Enabled       *bool               `json:"enabled,omitempty"`
	ImageOnly     *bool               `json:"image_only,omitempty"`
	Pricing       *overlayPricing     `json:"pricing,omitempty"`
	LongContext   *overlayLongContext `json:"long_context,omitempty"`
}

func parseCatalogOverlay(raw string) (*catalogOverlay, error) {
	eff := cloneRegistry(registry)
	hidden := map[string]bool{}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return &catalogOverlay{registry: eff, hidden: hidden}, nil
	}

	var entries []overlayEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return nil, err
	}
	for _, e := range entries {
		id := normalizeID(e.ID)
		if id == "" {
			continue
		}
		base, ok := eff[id]
		if !ok {
			if inferred, matched := fallbackByKeyword(id, eff); matched {
				base = inferred
			} else {
				base = DefaultSpec
			}
		}
		eff[id] = applyOverlay(base, e)
		if e.Enabled != nil && !*e.Enabled {
			hidden[id] = true
		} else {
			delete(hidden, id)
		}
	}
	return &catalogOverlay{registry: eff, hidden: hidden}, nil
}

func normalizeID(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

func cloneRegistry(src map[string]Spec) map[string]Spec {
	out := make(map[string]Spec, len(src))
	for id, spec := range src {
		out[id] = spec
	}
	return out
}

func applyOverlay(base Spec, e overlayEntry) Spec {
	if e.Name != "" {
		base.Name = e.Name
	}
	if e.ContextWindow > 0 {
		base.ContextWindow = e.ContextWindow
	}
	if e.MaxOutput > 0 {
		base.MaxOutputTokens = e.MaxOutput
	}
	if e.ImageOnly != nil {
		base.ImageOnly = *e.ImageOnly
	}
	if e.Pricing != nil {
		applyPricingOverlay(&base, *e.Pricing)
	}
	if e.LongContext != nil {
		applyLongContextOverlay(&base, *e.LongContext)
	}
	return base
}

func applyPricingOverlay(spec *Spec, p overlayPricing) {
	if p.Input > 0 {
		spec.InputPrice = p.Input
	}
	if p.CachedInput > 0 {
		spec.CachedPrice = p.CachedInput
	}
	if p.Output > 0 {
		spec.OutputPrice = p.Output
	}
	if p.PriorityInput > 0 {
		spec.InputPricePriority = p.PriorityInput
	}
	if p.PriorityCachedInput > 0 {
		spec.CachedPricePriority = p.PriorityCachedInput
	}
	if p.PriorityOutput > 0 {
		spec.OutputPricePriority = p.PriorityOutput
	}
	if p.FlexInput > 0 {
		spec.InputPriceFlex = p.FlexInput
	}
	if p.FlexCachedInput > 0 {
		spec.CachedPriceFlex = p.FlexCachedInput
	}
	if p.FlexOutput > 0 {
		spec.OutputPriceFlex = p.FlexOutput
	}
}

func applyLongContextOverlay(spec *Spec, p overlayLongContext) {
	if p.Threshold > 0 {
		spec.LongContextThreshold = p.Threshold
	}
	if p.InputMultiplier > 0 {
		spec.LongContextInputMultiplier = p.InputMultiplier
	}
	if p.CachedMultiplier > 0 {
		spec.LongContextCachedMultiplier = p.CachedMultiplier
	}
	if p.OutputMultiplier > 0 {
		spec.LongContextOutputMultiplier = p.OutputMultiplier
	}
}
