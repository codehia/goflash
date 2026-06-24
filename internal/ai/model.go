package ai

import (
	"fmt"

	"github.com/codehia/goflash/internal/utils"

	"github.com/zendev-sh/goai/provider"
	"github.com/zendev-sh/goai/provider/anthropic"
	"github.com/zendev-sh/goai/provider/deepseek"
	"github.com/zendev-sh/goai/provider/openai"
)

func NewModel(providerName string, modelName string) (provider.LanguageModel, error) {
	switch providerName {
	case "deepseek":
		return deepseek.Chat(modelName), nil
	case "anthropic":
		return anthropic.Chat(modelName), nil
	case "ollama":
		return openai.Chat(modelName), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}
}

func GetModel() (provider.LanguageModel, error) {
	providerName := GetProviderName()
	modelName, err := utils.GetEnv("MODEL_NAME")
	if err != nil {
		sugar.Errorw("Missing MODEL_NAME", "error", err)
		return nil, fmt.Errorf("unknown model name: %s", modelName)
	}
	return NewModel(providerName, modelName)
}

func GetProviderName() string {
	providerName, _ := utils.GetEnv("PROVIDER_NAME", "deepseek")
	return providerName
}
