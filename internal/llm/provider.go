package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"openpaws/internal/config"
	"openpaws/internal/model"
)

type Provider interface {
	ClassifyAccount(ctx context.Context, topic string, account model.Account) (model.Classification, error)
}

func NewProvider(name string, cfg config.Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mock":
		return MockProvider{}, nil
	case "openai":
		apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		if apiKey == "" {
			return nil, fmt.Errorf("provider openai requires OPENAI_API_KEY")
		}
		cfg.OpenAI.APIKey = apiKey
		return OpenAIProvider{
			BaseURL: cfg.OpenAI.BaseURL,
			APIKey:  cfg.OpenAI.APIKey,
			Model:   cfg.OpenAI.Model,
			Client:  &http.Client{Timeout: 30 * time.Second},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q", name)
	}
}

type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

func (p OpenAIProvider) ClassifyAccount(ctx context.Context, topic string, account model.Account) (model.Classification, error) {
	var lastErr error
	backoff := []time.Duration{100 * time.Millisecond, 500 * time.Millisecond, 2 * time.Second}

	for attempt := 0; attempt < 3; attempt++ {
		classification, err := p.classifyOnce(ctx, topic, account)
		if err != nil {
			lastErr = err
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return model.Classification{}, ctx.Err()
				case <-time.After(backoff[attempt]):
					continue
				}
			}
			continue
		}

		if err := validateClassification(classification); err != nil {
			lastErr = err
			if attempt < 2 {
				select {
				case <-ctx.Done():
					return model.Classification{}, ctx.Err()
				case <-time.After(backoff[attempt]):
					continue
				}
			}
			continue
		}

		return classification, nil
	}

	return defaultClassification(), fmt.Errorf("all retries failed, last error: %w", lastErr)
}

func (p OpenAIProvider) classifyOnce(ctx context.Context, topic string, account model.Account) (model.Classification, error) {
	payload := map[string]any{
		"model": p.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You classify creator alignment for advocacy outreach. Return only JSON with fields: alignment_label, alignment_confidence, receptivity_label, receptivity_score, opportunistic, hostile, rationale.",
			},
			{
				"role":    "user",
				"content": buildPrompt(topic, account),
			},
		},
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return model.Classification{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return model.Classification{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		return model.Classification{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return model.Classification{}, fmt.Errorf("openai returned status %s", resp.Status)
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return model.Classification{}, fmt.Errorf("decode response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return model.Classification{}, fmt.Errorf("openai returned no choices")
	}

	var classification model.Classification
	if err := json.Unmarshal([]byte(raw.Choices[0].Message.Content), &classification); err != nil {
		return model.Classification{}, fmt.Errorf("parse classification json: %w", err)
	}
	return classification, nil
}

func validateClassification(c model.Classification) error {
	validLabels := map[string]bool{
		"strong_animal_welfare":      true,
		"adjacent_progressive_cause": true,
		"neutral_general_interest":   true,
		"commercial_only":            true,
		"misaligned_or_hostile":      true,
	}
	if c.AlignmentLabel == "" {
		return fmt.Errorf("missing alignment_label")
	}
	if !validLabels[c.AlignmentLabel] {
		return fmt.Errorf("invalid alignment_label: %s", c.AlignmentLabel)
	}
	if c.AlignmentConfidence < 0 || c.AlignmentConfidence > 1 {
		return fmt.Errorf("invalid alignment_confidence: must be between 0 and 1")
	}
	if c.ReceptivityScore < 0 || c.ReceptivityScore > 1 {
		return fmt.Errorf("invalid receptivity_score: must be between 0 and 1")
	}
	return nil
}

func defaultClassification() model.Classification {
	return model.Classification{
		AlignmentLabel:      "neutral_general_interest",
		AlignmentConfidence: 0.50,
		ReceptivityLabel:    "medium",
		ReceptivityScore:    0.50,
		Rationale:           []string{"Classification unavailable - using uncertain default"},
	}
}

func buildPrompt(topic string, account model.Account) string {
	var postSnippets []string
	for i, post := range account.Posts {
		if i >= 5 {
			break
		}
		postSnippets = append(postSnippets, post.CaptionOrText)
	}

	// The prompt is intentionally compact so the API remains affordable when the
	// tool is run in batch mode over dozens or hundreds of candidate accounts.
	return fmt.Sprintf(
		"Campaign topic: %s\nHandle: %s\nBio: %s\nTopics claimed: %s\nSample posts: %s\nAssess genuine cause alignment, outreach receptivity, and signs of opportunism or hostility.",
		topic,
		account.Handle,
		account.Bio,
		strings.Join(account.TopicsClaimed, ", "),
		strings.Join(postSnippets, " || "),
	)
}
