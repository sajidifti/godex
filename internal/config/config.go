package config

import (
	"errors"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIKey string
	Model  string
}

func Load() (Config, error) {
	// .env is optional. Real environemnts takes priority over it.
	_ = godotenv.Load()

	key := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if key == "" {
		return Config{}, errors.New("OPENROUTER_API_KEY is not set")
	}

	model := strings.TrimSpace(os.Getenv("GODEX_MODEL"))
	if model == "" {
		model = "openrouter/free"
		// return Config{}, errors.New("GODEX_MODEL is not set")
	}
	return Config{APIKey: key, Model: model}, nil
}
