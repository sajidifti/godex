package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIKey    string
	Model     string
	Reasoning bool
}

func Load() (Config, error) {
	// .env is optional. Real environments take priority over it.
	_ = godotenv.Load()

	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		return Config{}, errors.New("OPENROUTER_API_KEY is not set")
	}

	model := strings.TrimSpace(os.Getenv("GODEX_MODEL"))
	if model == "" {
		model = "openrouter/free"
	}

	reasoning := true
	if v := strings.TrimSpace(os.Getenv("GODEX_REASONING")); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("GODEX_REASONING must be a boolean: %w", err)
		}
		reasoning = parsed
	}

	return Config{APIKey: key, Model: model, Reasoning: reasoning}, nil
}
