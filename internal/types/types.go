package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const (
	Model  = "deepseek-v4-flash"
	Stream = false
)

type Topic struct {
	Name string
}

type Node struct {
	Name     string `json:"name"`
	Notes    string `json:"notes,omitempty"`
	Children []Node `json:"children,omitempty"`
}

func LoadNode(path string) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Node{}, fmt.Errorf("LoadNode: read %q: %w", path, err)
	}
	var root Node
	if err := json.Unmarshal(data, &root); err != nil {
		return Node{}, fmt.Errorf("LoadNode: parse json: %w", err)
	}
	return root, nil
}

type Card struct {
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Examples  string `json:"examples"`
	TradeOffs string `json:"tradeoffs"`
	CardType  string `json:"card_type"`
}

func (c Card) Validate() error {
	validCardTypes := map[string]bool{
		"definition":  true,
		"mechanism":   true,
		"tradeoff":    true,
		"application": true,
	}
	if c.Question == "" {
		return errors.New("missing question")
	} else if c.Answer == "" {
		return errors.New("missing answer")
	} else if !validCardTypes[c.CardType] {
		return errors.New("invalid card type: " + c.CardType)
	}
	return nil
}

type Response struct {
	Tag     string   `json:"tag"`
	TagPath []string `json:"tag_path"`
	Cards   []Card   `json:"cards"`
}

func (r Response) Validate() error {
	if r.Tag == "" {
		return errors.New("tag field is empty")
	} else if len(r.TagPath) == 0 {
		return errors.New("tag_path field is empty")
	} else if len(r.Cards) == 0 {
		return errors.New("cards field is empty")
	}
	for _, card := range r.Cards {
		if err := card.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type APIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func NewMessage(content string, role string) (APIMessage, error) {
	if content == "" {
		return APIMessage{}, errors.New("empty strings are not allowed")
	}
	if role == "" {
		role = "user"
	}
	return APIMessage{Role: role, Content: content}, nil
}

type EvalResult struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
}

func (e EvalResult) Validate() error {
	if (e.Score > 5) || (e.Score < 0) {
		return errors.New("score is not in the 0-5 range")
	} else if e.Feedback == "" {
		return errors.New("missing feedback")
	}
	return nil
}

type RequestPayload struct {
	Model          string            `json:"model"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	ResponseFormat map[string]string `json:"response_format"`
	Messages       []APIMessage      `json:"messages"`
	Thinking       map[string]string `json:"thinking"`
	Stream         bool              `json:"stream"`
}

func NewRequestPayload(messages []APIMessage) (RequestPayload, error) {
	if len(messages) == 0 {
		return RequestPayload{}, errors.New("empty list of messages is not allowed")
	}

	thinking := map[string]string{"type": "disabled"}
	responseFormat := map[string]string{"type": "json_object"}

	return RequestPayload{
		Model:          Model,
		Temperature:    0.0,
		MaxTokens:      32000,
		ResponseFormat: responseFormat,
		Messages:       messages,
		Thinking:       thinking,
		Stream:         Stream,
	}, nil
}

type APIResponseMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type APIResponse struct {
	Choices []struct {
		Message APIResponseMessage `json:"message"`
	} `json:"choices"`
}

func (r APIResponse) FirstMessage() (APIResponseMessage, error) {
	if len(r.Choices) == 0 {
		return APIResponseMessage{}, errors.New("no choices in response")
	}
	return r.Choices[0].Message, nil
}

type Config struct {
	APIKey  string
	BaseURL string
}

func NewConfig() (Config, error) {
	cfg := Config{
		APIKey:  os.Getenv("DEEPSEEK_API_KEY"),
		BaseURL: os.Getenv("DEEPSEEK_BASE_URL"),
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("DEEPSEEK_API_KEY is not set")
	}
	if cfg.BaseURL == "" {
		return Config{}, errors.New("DEEPSEEK_BASE_URL is not set")
	}
	return cfg, nil
}
