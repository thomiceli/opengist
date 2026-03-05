package gist

import (
	"strings"

	"github.com/thomiceli/opengist/internal/ai"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/web/context"
)

// RecommendTopics handles AI-powered topic recommendations
func RecommendTopics(ctx *context.Context) error {
	// Get AI settings from database
	aiEnabled, _ := db.GetSetting(db.SettingAIEnabled)
	aiAPIType, _ := db.GetSetting(db.SettingAIAPIType)
	aiBaseURL, _ := db.GetSetting(db.SettingAIBaseURL)
	aiAPIKey, _ := db.GetSetting(db.SettingAIAPIKey)
	aiModel, _ := db.GetSetting(db.SettingAIModel)
	aiSystemPrompt, _ := db.GetSetting(db.SettingAISystemPrompt)
	aiUserPrompt, _ := db.GetSetting(db.SettingAIUserPrompt)

	// Check if AI is enabled
	if aiEnabled != "1" {
		return ctx.JSON(200, map[string]interface{}{
			"success": false,
			"error":   "AI topic recommendation is not enabled",
		})
	}

	// Create AI service
	aiService := ai.NewAIService(ai.AIServiceConfig{
		Enabled:            aiEnabled == "1",
		APIType:            aiAPIType,
		BaseURL:            aiBaseURL,
		APIKey:             aiAPIKey,
		Model:              aiModel,
		SystemPrompt:       aiSystemPrompt,
		UserPromptTemplate: aiUserPrompt,
	})

	// Get form data
	title := ctx.FormValue("title")
	description := ctx.FormValue("description")
	content := ctx.FormValue("content")
	filenamesStr := ctx.FormValue("filenames")

	// Parse filenames
	var filenames []string
	if filenamesStr != "" {
		filenames = strings.Split(filenamesStr, ",")
	}

	// Get recommendations
	topics, err := aiService.RecommendTopics(title, description, content, filenames)
	if err != nil {
		return ctx.JSON(200, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.JSON(200, map[string]interface{}{
		"success": true,
		"topics":  topics,
	})
}
