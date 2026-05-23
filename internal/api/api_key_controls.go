package api

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/usage"
	geminihandler "github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers/gemini"
	"github.com/tidwall/gjson"
)

func (s *Server) authAndAPIKeyControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authenticateRequest(c, s.accessManager) {
			return
		}
		if !s.enforceAPIKeyControls(c) {
			return
		}
		c.Next()
	}
}

func (s *Server) enforceAPIKeyControls(c *gin.Context) bool {
	if s == nil || c == nil {
		return true
	}
	cfg := s.cfg
	if cfg == nil || len(cfg.APIKeyControls) == 0 {
		return true
	}
	apiKey := strings.TrimSpace(c.GetString("apiKey"))
	if apiKey == "" {
		return true
	}
	control := findAPIKeyControl(cfg, apiKey)
	if control == nil {
		return true
	}
	if control.Enabled != nil && !*control.Enabled {
		abortAPIKeyControl(c, http.StatusForbidden, "api_key_disabled", "API key is disabled")
		return false
	}
	if !withinAPIKeyBudget(control, usage.GetRequestStatistics()) {
		abortAPIKeyControl(c, http.StatusTooManyRequests, "api_key_budget_exceeded", "API key usage budget exceeded")
		return false
	}
	modelName, ok := extractRequestModel(c)
	if ok && modelName != "" && !apiKeyModelAllowed(control, modelName) {
		abortAPIKeyControl(c, http.StatusForbidden, "model_not_allowed", "Model is not allowed for this API key")
		return false
	}
	return true
}

func findAPIKeyControl(cfg *config.Config, apiKey string) *config.APIKeyControl {
	if cfg == nil || apiKey == "" {
		return nil
	}
	for i := range cfg.APIKeyControls {
		key := strings.TrimSpace(cfg.APIKeyControls[i].APIKey)
		if key == "" {
			key = strings.TrimSpace(cfg.APIKeyControls[i].Key)
		}
		if key == apiKey {
			return &cfg.APIKeyControls[i]
		}
	}
	return nil
}

func withinAPIKeyBudget(control *config.APIKeyControl, stats *usage.RequestStatistics) bool {
	if control == nil || control.Unlimited {
		return true
	}
	if control.MaxRequests <= 0 && control.MaxInputTokens <= 0 && control.MaxTotalTokens <= 0 {
		return true
	}
	if stats == nil {
		return true
	}
	key := strings.TrimSpace(control.APIKey)
	if key == "" {
		key = strings.TrimSpace(control.Key)
	}
	if key == "" {
		return true
	}
	snapshot := stats.Snapshot()
	apiStats, ok := snapshot.APIs[key]
	if !ok {
		return true
	}
	if control.MaxRequests > 0 && apiStats.TotalRequests >= control.MaxRequests {
		return false
	}
	if control.MaxInputTokens > 0 && apiStats.TotalInputTokens >= control.MaxInputTokens {
		return false
	}
	if control.MaxTotalTokens > 0 && apiStats.TotalTokens >= control.MaxTotalTokens {
		return false
	}
	return true
}

func extractRequestModel(c *gin.Context) (string, bool) {
	if c == nil || c.Request == nil {
		return "", false
	}
	if model := extractGeminiModelFromPath(c.Request.URL.Path); model != "" {
		return model, true
	}
	if queryModel := strings.TrimSpace(c.Query("model")); queryModel != "" {
		return queryModel, true
	}
	if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut && c.Request.Method != http.MethodPatch {
		return "", false
	}
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "json") {
		return "", false
	}
	if c.Request.Body == nil {
		return "", false
	}
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(nil))
		return "", false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))
	if len(bytes.TrimSpace(rawBody)) == 0 {
		return "", false
	}
	modelResult := gjson.GetBytes(rawBody, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String {
		return "", false
	}
	return strings.TrimSpace(modelResult.String()), true
}

func extractGeminiModelFromPath(path string) string {
	const marker = "/models/"
	idx := strings.Index(path, marker)
	if idx < 0 {
		return ""
	}
	model := path[idx+len(marker):]
	if model == "" {
		return ""
	}
	if cut := strings.IndexAny(model, ":/?#"); cut >= 0 {
		model = model[:cut]
	}
	return strings.TrimPrefix(strings.TrimSpace(model), "models/")
}

func (s *Server) filterModelsForAPIKey(c *gin.Context, models []map[string]any) []map[string]any {
	if len(models) == 0 || s == nil || s.cfg == nil || c == nil {
		return models
	}
	control := findAPIKeyControl(s.cfg, strings.TrimSpace(c.GetString("apiKey")))
	if control == nil {
		return models
	}
	filtered := make([]map[string]any, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		if apiKeyModelAllowed(control, modelNameFromMap(model)) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func (s *Server) geminiModelsHandler(handler *geminihandler.GeminiAPIHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if handler == nil {
			c.JSON(http.StatusOK, gin.H{"models": []map[string]any{}})
			return
		}
		rawModels := s.filterModelsForAPIKey(c, handler.Models())
		normalizedModels := make([]map[string]any, 0, len(rawModels))
		defaultMethods := []string{"generateContent"}
		for _, model := range rawModels {
			normalizedModel := make(map[string]any, len(model))
			for k, v := range model {
				normalizedModel[k] = v
			}
			if name, ok := normalizedModel["name"].(string); ok && name != "" {
				if !strings.HasPrefix(name, "models/") {
					normalizedModel["name"] = "models/" + name
				}
				if displayName, _ := normalizedModel["displayName"].(string); displayName == "" {
					normalizedModel["displayName"] = name
				}
				if description, _ := normalizedModel["description"].(string); description == "" {
					normalizedModel["description"] = name
				}
			}
			if _, ok := normalizedModel["supportedGenerationMethods"]; !ok {
				normalizedModel["supportedGenerationMethods"] = defaultMethods
			}
			normalizedModels = append(normalizedModels, normalizedModel)
		}
		c.JSON(http.StatusOK, gin.H{"models": normalizedModels})
	}
}

func modelNameFromMap(model map[string]any) string {
	for _, key := range []string{"id", "name"} {
		value, ok := model[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimPrefix(strings.TrimSpace(value), "models/")
		}
	}
	return ""
}

func apiKeyModelAllowed(control *config.APIKeyControl, model string) bool {
	if control == nil {
		return true
	}
	model = strings.TrimPrefix(strings.TrimSpace(model), "models/")
	if model == "" {
		return true
	}
	for _, pattern := range control.ExcludedModels {
		if modelPatternMatches(model, pattern) {
			return false
		}
	}
	if len(control.Models) == 0 {
		return true
	}
	for _, pattern := range control.Models {
		if modelPatternMatches(model, pattern) {
			return true
		}
	}
	return false
}

func modelPatternMatches(model, pattern string) bool {
	model = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(model, "models/")))
	pattern = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(pattern, "models/")))
	if model == "" || pattern == "" {
		return false
	}
	if pattern == "*" || pattern == model {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	parts := strings.Split(pattern, "*")
	position := 0
	if parts[0] != "" {
		if !strings.HasPrefix(model, parts[0]) {
			return false
		}
		position = len(parts[0])
	}
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		next := strings.Index(model[position:], part)
		if next < 0 {
			return false
		}
		position += next + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(model, last)
}

func abortAPIKeyControl(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    "api_key_access_error",
			"code":    code,
		},
	})
}
