package gateway

import "strings"

func isGeminiImageModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "gemini-") && strings.Contains(model, "-image")
}
