package gateway

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

const tinyPNGDataURL = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestBuildGeminiImageChatRequestBodyTextOnly(t *testing.T) {
	body, err := buildGeminiImageChatRequestBody("gemini-2.5-flash-image", &imagesRequest{Prompt: "a cat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content := gjson.GetBytes(body, "messages.0.content")
	if content.Type != gjson.String || content.String() != "a cat" {
		t.Fatalf("纯文生图 content 应保持字符串, got: %s", content.Raw)
	}
	if got := gjson.GetBytes(body, "modalities").Raw; !strings.Contains(got, "image") {
		t.Fatalf("modalities 缺 image: %s", got)
	}
}

func TestBuildGeminiImageChatRequestBodyWithImages(t *testing.T) {
	body, err := buildGeminiImageChatRequestBody("gemini-2.5-flash-image", &imagesRequest{
		IsEdit: true,
		Prompt: "make it yellow",
		Images: []string{tinyPNGDataURL},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content := gjson.GetBytes(body, "messages.0.content")
	if !content.IsArray() {
		t.Fatalf("图生图 content 应是分段数组, got: %s", content.Raw)
	}
	parts := content.Array()
	if len(parts) != 2 {
		t.Fatalf("期望 text+image 两个分段, got %d", len(parts))
	}
	if parts[0].Get("type").String() != "text" || !strings.Contains(parts[0].Get("text").String(), "make it yellow") {
		t.Fatalf("首段应为 prompt 文本: %s", parts[0].Raw)
	}
	if parts[1].Get("type").String() != "image_url" {
		t.Fatalf("次段应为 image_url: %s", parts[1].Raw)
	}
	if url := parts[1].Get("image_url.url").String(); !strings.HasPrefix(url, "data:image/") {
		t.Fatalf("参考图应转成 data URL: %.60s", url)
	}
}

func TestBuildGeminiImageChatRequestBodyRejectsMask(t *testing.T) {
	_, err := buildGeminiImageChatRequestBody("gemini-2.5-flash-image", &imagesRequest{
		IsEdit: true,
		Prompt: "fix",
		Images: []string{tinyPNGDataURL},
		Mask:   tinyPNGDataURL,
	})
	if err == nil || !strings.Contains(err.Error(), "mask") {
		t.Fatalf("mask 应显式报错, got: %v", err)
	}
}

func TestNormalizeGeminiVideoParts(t *testing.T) {
	body := []byte(`{"model":"gemini-3.5-flash","messages":[{"role":"user","content":[` +
		`{"type":"text","text":"describe"},` +
		`{"type":"video_url","video_url":{"url":"data:video/mp4;base64,AAAA"}}]}]}`)
	out := normalizeGeminiVideoParts(body)
	part := gjson.GetBytes(out, "messages.0.content.1")
	if part.Get("type").String() != "image_url" {
		t.Fatalf("video_url 分段应改写成 image_url: %s", part.Raw)
	}
	if part.Get("image_url.url").String() != "data:video/mp4;base64,AAAA" {
		t.Fatalf("URL 应原样保留: %s", part.Raw)
	}
	if part.Get("video_url").Exists() {
		t.Fatalf("改写后不应残留 video_url 键: %s", part.Raw)
	}
	if text := gjson.GetBytes(out, "messages.0.content.0.text").String(); text != "describe" {
		t.Fatalf("text 分段不应被改动: %s", text)
	}
}

func TestNormalizeGeminiVideoPartsStringRef(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":[{"type":"video_url","video_url":"https://example.com/v.mp4"}]}]}`)
	out := normalizeGeminiVideoParts(body)
	part := gjson.GetBytes(out, "messages.0.content.0")
	if part.Get("type").String() != "image_url" || part.Get("image_url.url").String() != "https://example.com/v.mp4" {
		t.Fatalf("字符串形式 video_url 也应改写: %s", part.Raw)
	}
}

func TestNormalizeGeminiVideoPartsNoop(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"plain text"}]}`)
	if out := normalizeGeminiVideoParts(body); string(out) != string(body) {
		t.Fatalf("无 video_url 的请求体不应被改动")
	}
}

func TestIsGeminiModel(t *testing.T) {
	for model, want := range map[string]bool{
		"gemini-3.5-flash":       true,
		"Gemini-3.1-Pro-Preview": true,
		"gemini-2.5-flash-image": true,
		"gpt-5.5":                false,
		"":                       false,
	} {
		if got := isGeminiModel(model); got != want {
			t.Fatalf("isGeminiModel(%q)=%v, want %v", model, got, want)
		}
	}
}
