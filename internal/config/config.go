package config

type Config struct {
	Weights             Weights
	PlatformMultipliers map[string]float64
	LowConfidenceFloor  float64
	OpenAI              OpenAIConfig
}

type Weights struct {
	CauseAlignment float64
	Authenticity   float64
	Receptivity    float64
	Reach          float64
}

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

func Default() Config {
	return Config{
		Weights: Weights{
			CauseAlignment: 0.40,
			Authenticity:   0.25,
			Receptivity:    0.20,
			Reach:          0.15,
		},
		PlatformMultipliers: map[string]float64{
			"instagram": 1.0,
			"x":         0.8,
		},
		LowConfidenceFloor: 0.45,
		OpenAI: OpenAIConfig{
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4.1-mini",
		},
	}
}
