package config

import (
	"fmt" // Add fmt import for error wrapping

	"github.com/spf13/viper"
)

// PricingInfo holds cost details per token for a specific model.
type PricingInfo struct {
	InputPerToken  float64 `mapstructure:"input_per_token"`
	OutputPerToken float64 `mapstructure:"output_per_token"`
}

type Config struct {
	Database struct {
		Primary struct {
			DSN string
		}
		// Vector struct definition (Postgres only)
		Vector struct {
			DSN string `mapstructure:"DSN"` // DSN for Postgres vector store
		}
	}
	Embedding struct {
		Model           string `mapstructure:"model"`
		OpenaiApiKey    string `mapstructure:"openai_api_key"`
		GoogleApiKey    string `mapstructure:"google_api_key"`
		GeminiModelName string `mapstructure:"gemini_model_name"`
		Dimension       int    `mapstructure:"dimension"`
		UseBatchAPI     bool   `mapstructure:"use_batch_api"` // Add field for batch API toggle
	}
	Search struct {
		DefaultLimit int
	}

	Chunking struct { // Add Chunking struct
		MaxTokens int `mapstructure:"max_tokens"`
		Overlap   int `mapstructure:"overlap"`
	} `mapstructure:"chunking"` // Add mapstructure tag

	// Add Categorization struct back
	Categorization struct {
		Type           string `mapstructure:"type"`            // "llm" or "none"
		Provider       string `mapstructure:"provider"`        // "openai", "gemini" (if type is "llm")
		Model          string `mapstructure:"model"`           // Model name for the provider
		PromptTemplate string `mapstructure:"prompt_template"` // Path to prompt template file or the template itself
		AutoApplyTags  bool   `mapstructure:"auto_apply_tags"` // Add this line
	}

	Summarization struct {
		Enabled  bool   `mapstructure:"enabled"`
		Provider string `mapstructure:"provider"`
		Model    string `mapstructure:"model"`
		Prompt   string `mapstructure:"prompt"`
	}

	RAG struct { // Add RAG struct
		Enabled  bool   `mapstructure:"enabled"`
		Provider string `mapstructure:"provider"` // e.g., "gemini", "openai"
		Model    string `mapstructure:"model"`    // Model for generation
		Prompt   string `mapstructure:"prompt"`   // Path/content for RAG prompt
	} `mapstructure:"rag"` // Add mapstructure tag

	Redis struct {
		Address  string
		Password string `mapstructure:"password"` // Add password field
		DB       int    `mapstructure:"db"`       // Add DB field
	}

	Worker struct {
		Concurrency int            `mapstructure:"concurrency"`
		Queues      map[string]int `mapstructure:"queues"`
	}

	// Pricing: map[provider][model] = struct{input_per_token, output_per_token}
	Pricing map[string]map[string]PricingInfo `mapstructure:"pricing"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".") // Look for config.yaml in the current directory

	// --- Environment Variable Binding ---
	// Allow Viper to read environment variables
	viper.AutomaticEnv()
	// Optional: Set a prefix for environment variables to avoid conflicts
	// viper.SetEnvPrefix("MIMIR")
	// Optional: Define how to map config keys to env var names (e.g., embedding.openai_api_key -> MIMIR_EMBEDDING_OPENAI_API_KEY)
	// viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Explicitly bind the OPENAI_API_KEY environment variable to the config field.
	// This allows setting the key via env var without needing a prefix or specific naming convention.
	viper.BindEnv("embedding.openai_api_key", "OPENAI_API_KEY")
	// --- End Environment Variable Binding ---

	if err := viper.ReadInConfig(); err != nil {
		// It's okay if the config file doesn't exist, Viper might rely solely on env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found; proceed, relying on defaults/env vars.
		// Log this? log.Println("Config file not found, using defaults/env vars.")
	} else {
		// Config file was found and read successfully
		// Log this? log.Println("Using config file:", viper.ConfigFileUsed())
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	// The config variable is already declared and unmarshalled above.
	// Remove the redundant declaration and unmarshal call.
	return &config, nil
}
