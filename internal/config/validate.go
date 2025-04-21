package config

import (
	"errors"
	"fmt"
)

/*
Enhance configuration validation to ensure all required fields are checked,
especially for enabled providers/features. This covers:
- Database DSNs
- Embedding providers (openai, local, etc.)
- Redis
- Worker
- Chunking
- Categorization
- Summarization
- Pricing (if present)
*/

func (c *Config) Validate() error {
	// Database config
	if c.Database.Primary.DSN == "" {
		return errors.New("database.primary.DSN is required")
	}
	if c.Database.Vector.DSN == "" {
		return errors.New("database.vector.DSN is required")
	}

	// Embedding config validation based on the actual struct fields
	// Note: The current Config.Embedding struct seems simplified.
	// This validation reflects the fields present in config.go.
	// If OpenAI is intended to be used (e.g., via Batch API or implicitly), check its key.
	if c.Embedding.UseBatchAPI && c.Embedding.OpenaiApiKey == "" {
		// Assuming Batch API currently implies OpenAI
		return errors.New("embedding.openai_api_key is required when embedding.use_batch_api is true")
	}
	// Add check for Google API key if Gemini model is specified
	if c.Embedding.GeminiModelName != "" && c.Embedding.GoogleApiKey == "" {
		return errors.New("embedding.google_api_key is required when embedding.gemini_model_name is set")
	}

	if c.Embedding.Dimension <= 0 {
		return errors.New("embedding.dimension must be a positive integer")
	}

	// Redis config
	if c.Redis.Address == "" {
		return errors.New("redis.address is required")
	}

	// Worker config
	if c.Worker.Concurrency <= 0 {
		return errors.New("worker.concurrency must be a positive integer")
	}
	if len(c.Worker.Queues) == 0 {
		return errors.New("worker.queues must define at least one queue")
	}
	for name, priority := range c.Worker.Queues {
		if name == "" {
			return errors.New("worker.queues contains an empty queue name")
		}
		if priority <= 0 {
			return fmt.Errorf("worker.queues priority for queue '%s' must be positive", name)
		}
	}

	// Chunking config
	if c.Chunking.MaxTokens <= 0 {
		return errors.New("chunking.max_tokens must be positive")
	}
	if c.Chunking.Overlap < 0 || c.Chunking.Overlap >= c.Chunking.MaxTokens {
		return fmt.Errorf("chunking.overlap (%d) must be non-negative and less than max_tokens (%d)", c.Chunking.Overlap, c.Chunking.MaxTokens)
	}

	// Categorization config
	if c.Categorization.AutoApplyTags {
		if c.Categorization.Provider == "" {
			return errors.New("categorization.provider is required when auto_apply_tags is true")
		}
		if c.Categorization.Model == "" {
			return errors.New("categorization.model is required when auto_apply_tags is true")
		}
	}

	// Summarization config
	if c.Summarization.Enabled {
		if c.Summarization.Provider == "" {
			return errors.New("summarization.provider is required when summarization is enabled")
		}
		if c.Summarization.Model == "" {
			return errors.New("summarization.model is required when summarization is enabled")
		}
	}

	// Pricing config (optional, but if present, must be valid)
	// if c.Pricing != nil {
	// 	for provider, models := range c.Pricing {
	// 		if provider == "" {
	// 			return errors.New("pricing contains an empty provider name")
	// 		}
	// 		for model, price := range models {
	// 			if model == "" {
	// 				return fmt.Errorf("pricing for provider '%s' contains an empty model name", provider)
	// 			}
	// 			if price.InputPerToken < 0 || price.OutputPerToken < 0 {
	// 				return fmt.Errorf("pricing for provider '%s', model '%s' has negative token cost", provider, model)
	// 			}
	// 		}
	// 	}
	// }

	return nil
}
