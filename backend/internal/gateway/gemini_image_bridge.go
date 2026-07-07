package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

func shouldBridgeGeminiImageRequest(reqPath, model string, isEdit bool) bool {
	if isEdit || !strings.HasSuffix(reqPath, "/images/generations") {
		return false
	}
	return isGeminiImageModel(model)
}

func isGeminiImageModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(model, "gemini-") && strings.Contains(model, "-image")
}

func (g *OpenAIGateway) forwardGeminiImageViaGenerateContent(
	ctx context.Context,
	req *sdk.ForwardRequest,
	imgReq *imagesRequest,
	model string,
	start time.Time,
) (sdk.ForwardOutcome, error) {
	if imgReq == nil {
		body := jsonError("Images 请求解析失败")
		return sdk.ForwardOutcome{
			Kind:     sdk.OutcomeClientError,
			Upstream: sdk.UpstreamResponse{StatusCode: http.StatusBadRequest, Headers: jsonContentHeaders(), Body: body},
			Reason:   "Images 请求解析失败",
			Duration: time.Since(start),
		}, nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = strings.TrimSpace(imgReq.Model)
	}
	if model == "" {
		body := jsonError("model is required")
		return sdk.ForwardOutcome{
			Kind:     sdk.OutcomeClientError,
			Upstream: sdk.UpstreamResponse{StatusCode: http.StatusBadRequest, Headers: jsonContentHeaders(), Body: body},
			Reason:   "model is required",
			Duration: time.Since(start),
		}, nil
	}

	n := imgReq.N
	if n <= 0 {
		n = 1
	}
	data := make([]map[string]any, 0, n)
	for i := 0; i < n; i++ {
		body, err := buildGeminiGenerateContentRequest(imgReq)
		if err != nil {
			errBody := jsonError(err.Error())
			return sdk.ForwardOutcome{
				Kind:     sdk.OutcomeClientError,
				Upstream: sdk.UpstreamResponse{StatusCode: http.StatusBadRequest, Headers: jsonContentHeaders(), Body: errBody},
				Reason:   err.Error(),
				Duration: time.Since(start),
			}, nil
		}
		upstreamURL, err := buildGeminiGenerateContentURL(req.Account, model)
		if err != nil {
			reason := fmt.Sprintf("构建 Gemini 图片上游 URL 失败: %v", err)
			return transientOutcome(reason), fmt.Errorf("%s", reason)
		}
		upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(body))
		if err != nil {
			reason := fmt.Sprintf("构建 Gemini 图片上游请求失败: %v", err)
			return transientOutcome(reason), fmt.Errorf("%s", reason)
		}
		upstreamReq.Header.Set("Content-Type", "application/json")
		upstreamReq.Header.Set("Accept", "application/json")
		setAuthHeaders(upstreamReq, req.Account)

		resp, err := g.buildHTTPClient(req.Account).Do(upstreamReq)
		if err != nil {
			reason := fmt.Sprintf("请求 Gemini 图片上游失败: %v", err)
			return transientOutcome(reason), fmt.Errorf("%s", reason)
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			reason := fmt.Sprintf("读取 Gemini 图片上游响应失败: %v", readErr)
			return transientOutcome(reason), fmt.Errorf("%s", reason)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			errDetail := gjson.GetBytes(respBody, "error.message").String()
			if errDetail == "" {
				errDetail = truncate(string(respBody), 300)
			}
			outcome := failureOutcome(resp.StatusCode, respBody, resp.Header.Clone(), errDetail, extractRetryAfterHeader(resp.Header))
			outcome.Duration = time.Since(start)
			return outcome, nil
		}

		items, err := geminiImageDataFromGenerateContent(respBody)
		if err != nil {
			reason := "Gemini 图片响应中未包含可用图片: " + err.Error()
			return sdk.ForwardOutcome{
				Kind:     sdk.OutcomeUpstreamTransient,
				Upstream: sdk.UpstreamResponse{StatusCode: http.StatusBadGateway, Headers: jsonContentHeaders(), Body: jsonError(reason)},
				Reason:   reason,
				Duration: time.Since(start),
			}, nil
		}
		data = append(data, items...)
	}

	payload := map[string]any{
		"created": time.Now().Unix(),
		"data":    data,
		"model":   model,
	}
	respBody, err := json.Marshal(payload)
	if err != nil {
		reason := fmt.Sprintf("编码 Gemini 图片响应失败: %v", err)
		return transientOutcome(reason), fmt.Errorf("%s", reason)
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     jsonContentHeaders(),
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}
	return g.handleImagesResponse(resp, req.Writer, nil, start, model, imgReq.Size)
}

func buildGeminiGenerateContentURL(account *sdk.Account, model string) (string, error) {
	if account == nil {
		return "", fmt.Errorf("缺少账号")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(account.Credentials["base_url"]), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(account.Credentials["endpoint_url"]), "/")
	}
	if baseURL == "" {
		return "", fmt.Errorf("缺少 base_url")
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("base_url 格式错误")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	for strings.HasSuffix(u.Path, "/v1") || strings.HasSuffix(u.Path, "/v1beta") {
		u.Path = strings.TrimRight(strings.TrimSuffix(strings.TrimSuffix(u.Path, "/v1beta"), "/v1"), "/")
	}
	modelPath := "/v1beta/models/" + url.PathEscape(strings.TrimSpace(model)) + ":generateContent"
	u.Path = strings.TrimRight(u.Path, "/") + modelPath
	u.RawQuery = ""
	return u.String(), nil
}

func buildGeminiGenerateContentRequest(req *imagesRequest) ([]byte, error) {
	if req == nil {
		return nil, fmt.Errorf("请求不能为空")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt 不能为空")
	}
	generationConfig := map[string]any{
		"responseModalities": []string{"IMAGE"},
	}
	if imageConfig := geminiImageConfigForSize(req.Size); len(imageConfig) > 0 {
		generationConfig["imageConfig"] = imageConfig
	}
	body := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": prompt},
				},
			},
		},
		"generationConfig": generationConfig,
	}
	return json.Marshal(body)
}

func geminiImageConfigForSize(size string) map[string]any {
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "" || size == "auto" {
		size = "1024x1024"
	}
	switch size {
	case "1024x1024":
		return map[string]any{"aspectRatio": "1:1", "imageSize": "1K"}
	case "1536x1024":
		return map[string]any{"aspectRatio": "3:2", "imageSize": "1K"}
	case "1024x1536":
		return map[string]any{"aspectRatio": "2:3", "imageSize": "1K"}
	case "2048x2048":
		return map[string]any{"aspectRatio": "1:1", "imageSize": "2K"}
	case "2048x1152":
		return map[string]any{"aspectRatio": "16:9", "imageSize": "2K"}
	case "1152x2048":
		return map[string]any{"aspectRatio": "9:16", "imageSize": "2K"}
	case "3840x2160":
		return map[string]any{"aspectRatio": "16:9", "imageSize": "4K"}
	case "2160x3840":
		return map[string]any{"aspectRatio": "9:16", "imageSize": "4K"}
	default:
		return nil
	}
}

func geminiImageDataFromGenerateContent(body []byte) ([]map[string]any, error) {
	var items []map[string]any
	candidates := gjson.GetBytes(body, "candidates")
	if !candidates.Exists() || !candidates.IsArray() {
		return nil, fmt.Errorf("响应缺少 candidates 数组")
	}
	for _, candidate := range candidates.Array() {
		for _, part := range candidate.Get("content.parts").Array() {
			inlineData := part.Get("inlineData")
			if !inlineData.Exists() {
				inlineData = part.Get("inline_data")
			}
			if !inlineData.Exists() {
				continue
			}
			data := strings.TrimSpace(inlineData.Get("data").String())
			if data == "" {
				continue
			}
			item := map[string]any{"b64_json": data}
			if mimeType := strings.TrimSpace(firstNonEmptyString(inlineData.Get("mimeType").String(), inlineData.Get("mime_type").String())); mimeType != "" {
				item["mime_type"] = mimeType
			}
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("响应 parts 中没有 inlineData.data")
	}
	return items, nil
}

func jsonContentHeaders() http.Header {
	return http.Header{"Content-Type": []string{"application/json"}}
}
