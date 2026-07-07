package gateway

import (
	"fmt"
	"strings"
)

var imageModelSupportedSizes = map[string]map[string]struct{}{
	"gpt-image-2": {
		"auto": {}, "1024x1024": {}, "1536x1024": {}, "1024x1536": {},
		"1536x864": {}, "864x1536": {}, "1536x1152": {}, "1152x1536": {},
		"2048x2048": {}, "2048x1152": {}, "1152x2048": {}, "2048x1536": {},
		"1536x2048": {}, "2000x1600": {}, "1600x2000": {}, "3840x2160": {},
		"2160x3840": {}, "3360x1440": {}, "1440x3360": {},
	},
	"gemini-2.5-flash-image": {
		"1024x1024": {}, "1536x1024": {}, "1024x1536": {},
	},
	"gemini-3-pro-image": {
		"1024x1024": {}, "1536x1024": {}, "1024x1536": {},
		"2048x2048": {}, "2048x1152": {}, "1152x2048": {},
		"3840x2160": {}, "2160x3840": {},
	},
	"gemini-3-pro-image-preview": {
		"1024x1024": {}, "1536x1024": {}, "1024x1536": {},
		"2048x2048": {}, "2048x1152": {}, "1152x2048": {},
		"3840x2160": {}, "2160x3840": {},
	},
	"gemini-3.1-flash-image": {
		"1024x1024": {}, "1536x1024": {}, "1024x1536": {},
		"2048x2048": {}, "2048x1152": {}, "1152x2048": {},
	},
	"gemini-3.1-flash-image-preview": {
		"1024x1024": {}, "1536x1024": {}, "1024x1536": {},
		"2048x2048": {}, "2048x1152": {}, "1152x2048": {},
	},
	"gemini-3.1-flash-lite-image": {
		"1024x1024": {}, "1536x1024": {}, "1024x1536": {},
	},
}

func validateImageModelSize(model, size string) error {
	model = strings.ToLower(strings.TrimSpace(model))
	size = strings.ToLower(strings.TrimSpace(size))
	if model == "" || size == "" {
		return nil
	}
	allowed, ok := imageModelSupportedSizes[model]
	if !ok {
		return nil
	}
	if _, ok := allowed[size]; ok {
		return nil
	}
	return fmt.Errorf("模型 %s 不支持尺寸 %s", model, size)
}
