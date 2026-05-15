package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/codehia/goflash/internal/types"
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
