package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// APIType represents the type of AI API
type APIType string

const (
	APITypeOllama  APIType = "ollama"
	APITypeOpenAI  APIType = "openai"
)

type AIService struct {
	Enabled            bool
	APIType            APIType
	BaseURL            string
	APIKey             string
	Model              string
	SystemPrompt       string
	UserPromptTemplate string
}

type AIServiceConfig struct {
	Enabled            bool
	APIType            string
	BaseURL            string
	APIKey             string
	Model              string
	SystemPrompt       string
	UserPromptTemplate string
}

// ChatRequest represents the Ollama chat request
type OllamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  OllamaOptions   `json:"options,omitempty"`
}

// Message represents a chat message
type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Options represents Ollama specific options
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// ChatResponse represents the Ollama chat response
type OllamaChatResponse struct {
	Model     string        `json:"model"`
	CreatedAt string        `json:"created_at"`
	Message   OllamaMessage `json:"message"`
	Done      bool          `json:"done"`
}

// OpenAI Chat Completion types
type OpenAIChatRequest struct {
	Model       string                 `json:"model"`
	Messages    []OpenAIMessage        `json:"messages"`
	Stream      bool                   `json:"stream"`
	Temperature float64                `json:"temperature,omitempty"`
	ExtraParams map[string]interface{} `json:"-"` // Extra params merged into JSON
}

// MarshalJSON implements custom JSON marshaling to include extra params
func (r OpenAIChatRequest) MarshalJSON() ([]byte, error) {
	type Alias OpenAIChatRequest
	aux := make(map[string]interface{})
	
	// Marshal standard fields
	if r.Model != "" {
		aux["model"] = r.Model
	}
	aux["messages"] = r.Messages
	aux["stream"] = r.Stream
	if r.Temperature > 0 {
		aux["temperature"] = r.Temperature
	}
	
	// Merge extra params
	for k, v := range r.ExtraParams {
		aux[k] = v
	}
	
	return json.Marshal(aux)
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	Choices []OpenAIChoice `json:"choices"`
}

type OpenAIChoice struct {
	Message OpenAIMessage `json:"message"`
}

// Default prompts
const DefaultSystemPrompt = "You are a helpful assistant that suggests relevant topic tags for code snippets. Respond with ONLY 3-5 space-separated topic tags. Each tag should be lowercase, alphanumeric, and hyphenated if needed. Do not include any explanations or additional text."

const DefaultUserPromptTemplate = `Please suggest relevant topic tags for the following code snippet:

{{if .Title}}Title: {{.Title}}
{{end}}{{if .Description}}Description: {{.Description}}
{{end}}{{if .Filenames}}Filenames: {{.Filenames}}
{{end}}
Content:
` + "```" + `
{{.Content}}
` + "```" + `

Respond with ONLY 3-5 space-separated topic tags (e.g., python web-framework api).`

// PromptData holds the data for template rendering
type PromptData struct {
	Title       string
	Description string
	Content     string
	Filenames   string
}

// NewAIService creates a new AI service with the given configuration
func NewAIService(config AIServiceConfig) *AIService {
	if !config.Enabled || config.BaseURL == "" {
		return &AIService{
			Enabled: false,
		}
	}

	// Use default prompts if not provided
	systemPrompt := config.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}

	userPromptTemplate := config.UserPromptTemplate
	if userPromptTemplate == "" {
		userPromptTemplate = DefaultUserPromptTemplate
	}

	apiType := APIType(config.APIType)
	if apiType == "" {
		apiType = APITypeOllama
	}

	return &AIService{
		Enabled:            true,
		APIType:            apiType,
		BaseURL:            config.BaseURL,
		APIKey:             config.APIKey,
		Model:              config.Model,
		SystemPrompt:       systemPrompt,
		UserPromptTemplate: userPromptTemplate,
	}
}

// RecommendTopics generates topic recommendations based on the gist content
func (s *AIService) RecommendTopics(title, description, content string, filenames []string) ([]string, error) {
	if !s.Enabled {
		return nil, fmt.Errorf("AI service is not enabled")
	}

	// Build the user prompt
	userPrompt := s.buildUserPrompt(title, description, content, filenames)

	log.Info().Str("type", string(s.APIType)).Str("model", s.Model).Str("baseURL", s.BaseURL).Msg("Calling AI API for topic recommendation")

	// Call the appropriate API based on type
	var topics []string
	var err error

	switch s.APIType {
	case APITypeOpenAI:
		topics, err = s.callOpenAIAPI(userPrompt)
	default:
		topics, err = s.callOllamaAPI(userPrompt)
	}

	if err != nil {
		return nil, err
	}

	return topics, nil
}

// buildUserPrompt builds the user prompt using the template
func (s *AIService) buildUserPrompt(title, description, content string, filenames []string) string {
	// Truncate content if too long
	if len(content) > 2000 {
		content = content[:2000] + "..."
	}

	// Join filenames
	filenamesStr := strings.Join(filenames, ", ")

	// Simple template replacement
	prompt := s.UserPromptTemplate
	prompt = strings.ReplaceAll(prompt, "{{.Title}}", title)
	prompt = strings.ReplaceAll(prompt, "{{.Description}}", description)
	prompt = strings.ReplaceAll(prompt, "{{.Content}}", content)
	prompt = strings.ReplaceAll(prompt, "{{.Filenames}}", filenamesStr)

	// Handle conditional sections
	if title == "" {
		prompt = strings.ReplaceAll(prompt, "{{if .Title}}Title: {{.Title}}\n{{end}}", "")
	}
	if description == "" {
		prompt = strings.ReplaceAll(prompt, "{{if .Description}}Description: {{.Description}}\n{{end}}", "")
	}
	if len(filenames) == 0 {
		prompt = strings.ReplaceAll(prompt, "{{if .Filenames}}Filenames: {{.Filenames}}\n{{end}}", "")
	}

	return prompt
}

// callOllamaAPI makes the HTTP request to the Ollama API
func (s *AIService) callOllamaAPI(userPrompt string) ([]string, error) {
	reqBody := OllamaChatRequest{
		Model: s.Model,
		Messages: []OllamaMessage{
			{
				Role:    "system",
				Content: s.SystemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream: false,
		Options: OllamaOptions{
			Temperature: 0.7,
			TopP:        0.9,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build endpoint URL
	endpoint := strings.TrimSuffix(s.BaseURL, "/") + "/api/chat"

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().Int("status", resp.StatusCode).Str("body", string(body)).Msg("Ollama API error")
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var respBody OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Parse the topics from the response
	topicsStr := respBody.Message.Content
	topics := s.parseTopics(topicsStr)

	log.Debug().Strs("topics", topics).Msg("Ollama recommended topics")

	return topics, nil
}

// callOpenAIAPI makes the HTTP request to the OpenAI-compatible API
func (s *AIService) callOpenAIAPI(userPrompt string) ([]string, error) {
	reqBody := OpenAIChatRequest{
		Model: s.Model,
		Messages: []OpenAIMessage{
			{
				Role:    "system",
				Content: s.SystemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Stream:      false,
		Temperature: 0.7,
		ExtraParams: map[string]interface{}{
			"enable_thinking": false,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build endpoint URL
	endpoint := strings.TrimSuffix(s.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint = endpoint + "/chat/completions"
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error().Int("status", resp.StatusCode).Str("body", string(body)).Msg("OpenAI API error")
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var respBody OpenAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(respBody.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from API")
	}

	// Parse the topics from the response
	topicsStr := respBody.Choices[0].Message.Content
	topics := s.parseTopics(topicsStr)

	log.Debug().Strs("topics", topics).Msg("OpenAI recommended topics")

	return topics, nil
}

// parseTopics extracts topic tags from the AI response (space-separated)
func (s *AIService) parseTopics(response string) []string {
	// Split by whitespace (spaces, newlines, tabs)
	fields := strings.Fields(response)
	topics := make([]string, 0, len(fields))

	for _, field := range fields {
		// Clean up each topic
		topic := strings.TrimSpace(strings.ToLower(field))

		// Remove any leading/trailing quotes or special characters
		topic = strings.Trim(topic, "\"'`,.;:")

		// Skip empty topics
		if topic == "" {
			continue
		}

		// Validate topic format (alphanumeric and hyphens only)
		if isValidTopic(topic) {
			topics = append(topics, topic)
		}
	}

	return topics
}

// isValidTopic checks if a topic string is valid
func isValidTopic(topic string) bool {
	if len(topic) == 0 || len(topic) > 50 {
		return false
	}

	// Topic should start with alphanumeric and contain only alphanumeric and hyphens
	for i, r := range topic {
		if i == 0 {
			// First character must be alphanumeric
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
				return false
			}
		} else {
			// Subsequent characters can be alphanumeric or hyphen
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
	}

	return true
}

// GetDefaultSystemPrompt returns the default system prompt
func GetDefaultSystemPrompt() string {
	return DefaultSystemPrompt
}

// GetDefaultUserPromptTemplate returns the default user prompt template
func GetDefaultUserPromptTemplate() string {
	return DefaultUserPromptTemplate
}

// GetAPITypeOptions returns available API type options
func GetAPITypeOptions() []struct {
	Value string
	Label string
} {
	return []struct {
		Value string
		Label string
	}{
		{Value: "ollama", Label: "Ollama"},
		{Value: "openai", Label: "OpenAI / Compatible"},
	}
}
