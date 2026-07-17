package gateway

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// isGeminiModel 判断模型是否属于经 OpenAI 兼容协议中继的 Gemini 家族（chat 与图片）。
func isGeminiModel(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), "gemini")
}

// normalizeGeminiVideoParts 把 chat completions 消息里的 video_url 分段改写成
// image_url 分段（data URL / http URL 原样保留）。
//
// 背景：Gemini 上游走 new-api 中继时，image_url 里带 data:video/* 会被正确透传成
// inline 视频、模型能真实理解内容；而 video_url 分段会被中继静默丢弃——上游仍返
// 回 200，但模型看不到视频、直接凭空编造（实测确认）。这里统一改写，让两种客户
// 端写法都落在可用的通道上。
func normalizeGeminiVideoParts(body []byte) []byte {
	if len(body) == 0 || !bytes.Contains(body, []byte(`"video_url"`)) {
		return body
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body
	}
	out := body
	for mi, msg := range messages.Array() {
		content := msg.Get("content")
		if !content.IsArray() {
			continue
		}
		for ci, part := range content.Array() {
			if part.Get("type").String() != "video_url" {
				continue
			}
			ref := part.Get("video_url")
			url := ref.Get("url").String()
			if url == "" && ref.Type == gjson.String {
				url = ref.String()
			}
			if strings.TrimSpace(url) == "" {
				continue
			}
			path := fmt.Sprintf("messages.%d.content.%d", mi, ci)
			replaced, err := sjson.SetBytes(out, path, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": url},
			})
			if err != nil {
				continue
			}
			out = replaced
		}
	}
	return out
}
