package gateway

import (
	"encoding/json"
	"testing"
)

func TestBuildLocalModelsResponseOwnedByHopBase(t *testing.T) {
	outcome := buildLocalModelsResponse()
	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.Unmarshal(outcome.Upstream.Body, &payload); err != nil {
		t.Fatalf("decode models response: %v", err)
	}
	if len(payload.Data) == 0 {
		t.Fatal("models list is empty")
	}
	for _, item := range payload.Data {
		if item.OwnedBy != "hopbase" {
			t.Fatalf("model %s owned_by = %q, want hopbase", item.ID, item.OwnedBy)
		}
	}
}
