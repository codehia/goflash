package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/codehia/goflash/internal/types"
	"github.com/zendev-sh/goai"
	"github.com/zendev-sh/goai/provider"
)

func MakeRequest(payloadData types.RequestPayload, cfg types.Config) ([]byte, error) {
	payload, err := json.Marshal(payloadData)
	if err != nil {
		// sugar.Errorw("marshalling payload failed", "error", err)
		return nil, err
	}

	// sugar.Infow("sending request", "model", payloadData.Model)
	req, err := http.NewRequest("POST", cfg.BaseURL, bytes.NewReader(payload))
	if err != nil {
		// sugar.Errorw("failed to create request", "error", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		// sugar.Warnw("http request failed, retrying", "error", err)
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// sugar.Warnw("reading response body failed, retrying", "error", err)
		return nil, err
	}

	var apiResponse types.APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		// sugar.Errorw("failed to unmarshal api response", "error", err)
		return nil, err
	}

	msg, err := apiResponse.FirstMessage()
	if err != nil {
		// sugar.Errorw("failed to get first message", "error", err)
		return nil, err
	}

	return []byte(msg.Content), nil
}

type Validatable interface {
	Validate() error
}

func GenerateStructured[T Validatable](ctx context.Context, model provider.LanguageModel, providerName string, opts ...goai.Option) (T, error) {
	var zero T
	switch providerName {
	case "deepseek", "ollama":
		opts = append(opts, goai.WithProviderOptions(map[string]any{"structuredOutputs": false}))
	}
	var err error
	for range 3 {
		r, e := goai.GenerateObject[T](ctx, model, opts...)
		if e != nil {
			err = e
			continue
		}
		if e = r.Object.Validate(); e != nil {
			err = e
			continue
		}
		return r.Object, nil
	}
	return zero, err
}
