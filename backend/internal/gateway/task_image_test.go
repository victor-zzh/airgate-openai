package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

type imageTaskHost struct {
	t              *testing.T
	forwardHeaders http.Header
	updateCalls    int
}

func (h *imageTaskHost) Invoke(_ context.Context, req sdk.HostInvokeRequest) (*sdk.HostInvokeResponse, error) {
	h.t.Helper()
	switch req.Method {
	case hostMethodTasksUpdate:
		h.updateCalls++
		return &sdk.HostInvokeResponse{Status: "ok", Payload: map[string]any{}}, nil
	case hostMethodGatewayForward:
		h.forwardHeaders = headerFromPayload(req.Payload["headers"])
		return &sdk.HostInvokeResponse{
			Status: "ok",
			Payload: map[string]any{
				"status_code": http.StatusOK,
				"body":        `{"data":[{"b64_json":"AA=="}]}`,
			},
		}, nil
	case hostMethodAssetsStore:
		return &sdk.HostInvokeResponse{
			Status: "ok",
			Payload: map[string]any{
				"public_url": "/assets-runtime/generated/1/test.png",
				"object_key": "generated/1/test.png",
			},
		}, nil
	default:
		body, _ := json.Marshal(req.Payload)
		return nil, fmt.Errorf("unexpected method %s payload=%s", req.Method, body)
	}
}

func (h *imageTaskHost) InvokeStream(context.Context, sdk.HostStreamRequest) (sdk.HostStream, error) {
	return nil, fmt.Errorf("not used")
}

func TestExecuteImageTaskDeclaresOpenAIPlatformForCompatibleGeminiModel(t *testing.T) {
	host := &imageTaskHost{t: t}
	g := &OpenAIGateway{host: host, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rt := &TaskRuntime{g: g, taskID: 208, logger: g.logger}

	err := executeImageTask(context.Background(), g, sdk.HostTask{
		ID:       208,
		UserID:   1,
		TaskType: taskTypeImageGenerate,
		Input: map[string]any{
			"prompt":   "生成一个小狗",
			"model":    "gemini-3-pro-image",
			"size":     "1024x1024",
			"group_id": int64(18),
		},
	}, rt, "/v1/images/generations")
	if err != nil {
		t.Fatalf("executeImageTask returned err: %v", err)
	}
	if got := host.forwardHeaders.Get("X-Airgate-Platform"); got != PluginPlatform {
		t.Fatalf("X-Airgate-Platform = %q, want %q; headers=%v", got, PluginPlatform, host.forwardHeaders)
	}
	if got := host.forwardHeaders.Get(taskExecHeader); !strings.EqualFold(got, "true") {
		t.Fatalf("%s = %q, want true", taskExecHeader, got)
	}
	if host.updateCalls < 2 {
		t.Fatalf("updateCalls = %d, want progress and complete updates", host.updateCalls)
	}
}
