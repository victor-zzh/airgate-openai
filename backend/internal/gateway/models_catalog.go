package gateway

import (
	"context"
	"time"

	"github.com/DouDOU-start/airgate-openai/backend/internal/model"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

const catalogRefreshInterval = 60 * time.Second

func (g *OpenAIGateway) runCatalogRefresh(ctx context.Context) {
	g.refreshCatalogOnce(ctx)
	ticker := time.NewTicker(catalogRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.refreshCatalogOnce(ctx)
		}
	}
}

func (g *OpenAIGateway) refreshCatalogOnce(ctx context.Context) {
	if g.host == nil {
		return
	}
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	payload, err := g.hostInvoke(fetchCtx, hostMethodModelsCatalog, map[string]interface{}{
		"platform": PluginPlatform,
	})
	if err != nil {
		g.logger.Warn("models_catalog_fetch_failed", sdk.LogFieldError, err)
		return
	}
	raw, _ := payload["catalog_json"].(string)
	stats, err := model.SetCatalogOverlayJSON(raw)
	if err != nil {
		g.logger.Warn("models_catalog_parse_failed", sdk.LogFieldError, err)
		return
	}
	g.logger.Debug("models_catalog_applied",
		"registry_size", stats.RegistrySize,
		"hidden_size", stats.HiddenSize,
	)
}
