package services

import (
	"context"
	"fmt"
	"log"
	// "sync" // Removed as struct definition moved to types.go
	"time"

	"github.com/pgvector/pgvector-go"
	// "mimir/internal/store" // Removed unused import
)

// --- Fallback Embedding Service Methods ---
// The FallbackEmbeddingService struct definition is in types.go

// NewFallbackEmbeddingService creates a new fallback service.
// Note: The struct definition is in types.go, this is the constructor.
func NewFallbackEmbeddingService(providers []EmbeddingProvider, strategy RetryStrategy) (*FallbackEmbeddingService, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("at least one embedding provider is required")
	}
	if strategy == nil {
		// Provide a default strategy if none is given
		strategy = &SimpleRetryStrategy{MaxAttempts: 3, BaseDelayMs: 100}
	}
	// Ensure all providers have the same dimension
	if len(providers) > 1 {
		dim := providers[0].Dimension()
		for i := 1; i < len(providers); i++ {
			if providers[i].Dimension() != dim {
				return nil, fmt.Errorf("all embedding providers must have the same dimension (provider %s has %d, expected %d)",
					providers[i].Name(), providers[i].Dimension(), dim)
			}
		}
	}

	return &FallbackEmbeddingService{
		Providers:      providers,
		ActiveProvider: 0, // Start with the first provider
		RetryStrategy:  strategy,
	}, nil
}

// Dimension returns the dimension of the currently active provider.
// Assumes all providers have the same dimension, enforced by constructor.
func (s *FallbackEmbeddingService) Dimension() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Providers) == 0 {
		log.Println("WARN: FallbackEmbeddingService has no providers, returning dimension 0")
		return 0
	}
	return s.Providers[s.ActiveProvider].Dimension()
}

// GenerateEmbedding tries providers with retries until one succeeds or all fail.
func (s *FallbackEmbeddingService) GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	s.mu.RLock()
	initialProviderIndex := s.ActiveProvider
	numProviders := len(s.Providers)
	if numProviders == 0 {
		s.mu.RUnlock()
		return pgvector.Vector{}, fmt.Errorf("no embedding providers configured")
	}
	s.mu.RUnlock()

	var lastErr error
	attempt := 0 // Track attempts across providers for the retry strategy

	for { // Loop indefinitely until success, context cancellation, or exhaustion
		s.mu.RLock()
		currentProviderIndex := s.ActiveProvider
		provider := s.Providers[currentProviderIndex]
		s.mu.RUnlock()

		log.Printf("Attempt %d: Trying provider %s (%s)", attempt+1, provider.Name(), provider.ModelName())
		vec, err := provider.GenerateEmbedding(ctx, text)

		// Check context cancellation immediately after the potentially long call
		if ctx.Err() != nil {
			log.Printf("Context cancelled after attempt with provider %s", provider.Name())
			return pgvector.Vector{}, fmt.Errorf("context cancelled during embedding generation: %w", ctx.Err())
		}

		if err == nil {
			// Success
			log.Printf("Provider %s succeeded.", provider.Name())
			return vec, nil
		}

		// Provider failed
		lastErr = fmt.Errorf("provider %s failed: %w", provider.Name(), err)
		log.Printf("WARN: Provider %s failed: %v", provider.Name(), err)

		// Decide whether to retry or switch based on strategy
		backoffMs := s.RetryStrategy.NextBackoff(attempt)
		if backoffMs < 0 { // Strategy says stop retrying (at this attempt level)
			log.Printf("Retry strategy indicates stopping retries for provider %s after attempt %d.", provider.Name(), attempt+1)

			// Switch to the next provider
			s.mu.Lock()
			nextProviderIndex := (s.ActiveProvider + 1) % numProviders
			// Check if we've cycled through all providers *since the last successful switch*
			if nextProviderIndex == initialProviderIndex {
				s.mu.Unlock()
				log.Printf("ERROR: Cycled through all providers. Embedding failed.")
				return pgvector.Vector{}, fmt.Errorf("all embedding providers failed after cycling through: last error: %w", lastErr)
			}
			s.ActiveProvider = nextProviderIndex
			log.Printf("Switching active provider to index %d: %s", nextProviderIndex, s.Providers[nextProviderIndex].Name())
			initialProviderIndex = nextProviderIndex // Reset the cycle detection start point
			s.mu.Unlock()

			attempt = 0 // Reset attempt count for the new provider cycle? Or keep global? Resetting for now.
			// Don't wait after switching, try the new provider immediately
			continue
		}

		// Wait before retrying (with the same provider)
		log.Printf("Waiting %dms before retrying with provider %s (attempt %d)", backoffMs, provider.Name(), attempt+1)
		select {
		case <-time.After(time.Duration(backoffMs) * time.Millisecond):
			attempt++ // Increment attempt count only if we waited and are retrying
			// Continue to the next attempt in the loop
		case <-ctx.Done():
			log.Printf("Context cancelled while waiting to retry provider %s", provider.Name())
			return pgvector.Vector{}, fmt.Errorf("context cancelled while waiting to retry: %w", ctx.Err())
		}
	}
}

// GenerateEmbeddings handles batch generation with fallback and retries.
// It attempts to use the active provider's batch method directly.
func (s *FallbackEmbeddingService) GenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	s.mu.RLock()
	initialProviderIndex := s.ActiveProvider
	numProviders := len(s.Providers)
	if numProviders == 0 {
		s.mu.RUnlock()
		return nil, fmt.Errorf("no embedding providers configured")
	}
	s.mu.RUnlock()

	var lastErr error
	attempt := 0 // Track attempts across providers for the retry strategy

	for { // Loop indefinitely until success, context cancellation, or exhaustion
		s.mu.RLock()
		currentProviderIndex := s.ActiveProvider
		provider := s.Providers[currentProviderIndex]
		s.mu.RUnlock()

		log.Printf("Attempt %d: Trying provider %s (%s) for batch size %d", attempt+1, provider.Name(), provider.ModelName(), len(texts))
		// Directly call the provider's batch method
		vecs, err := provider.GenerateEmbeddings(ctx, texts)

		// Check context cancellation immediately after the potentially long call
		if ctx.Err() != nil {
			log.Printf("Context cancelled after attempt with provider %s", provider.Name())
			return nil, fmt.Errorf("context cancelled during batch embedding generation: %w", ctx.Err())
		}

		if err == nil {
			// Success
			log.Printf("Provider %s succeeded for batch.", provider.Name())
			// Ensure the correct number of vectors was returned
			if len(vecs) != len(texts) {
				// This indicates a provider implementation issue
				log.Printf("ERROR: Provider %s returned %d vectors for %d texts", provider.Name(), len(vecs), len(texts))
				lastErr = fmt.Errorf("provider %s returned mismatched vector count (%d != %d)", provider.Name(), len(vecs), len(texts))
				// Treat this as a failure and try the next provider (or retry logic)
			} else {
				log.Printf("Successfully generated embeddings for batch of size %d using %s", len(texts), provider.Name())
				return vecs, nil // Success!
			}
		}

		// Provider failed or returned mismatched count
		if err != nil { // Log original error if it exists
			lastErr = fmt.Errorf("provider %s failed batch generation: %w", provider.Name(), err)
			log.Printf("WARN: Provider %s failed batch generation: %v", provider.Name(), err)
		} else { // Log mismatch error
			log.Printf("WARN: Provider %s returned mismatched vector count (%d != %d)", provider.Name(), len(vecs), len(texts))
		}

		// Decide whether to retry or switch based on strategy
		backoffMs := s.RetryStrategy.NextBackoff(attempt)
		if backoffMs < 0 { // Strategy says stop retrying (at this attempt level)
			log.Printf("Retry strategy indicates stopping retries for provider %s after attempt %d.", provider.Name(), attempt+1)

			// Switch to the next provider
			s.mu.Lock()
			nextProviderIndex := (s.ActiveProvider + 1) % numProviders
			// Check if we've cycled through all providers *since the last successful switch*
			if nextProviderIndex == initialProviderIndex {
				s.mu.Unlock()
				log.Printf("ERROR: Cycled through all providers. Batch embedding failed.")
				return nil, fmt.Errorf("all embedding providers failed batch generation after cycling through: last error: %w", lastErr)
			}
			s.ActiveProvider = nextProviderIndex
			log.Printf("Switching active provider to index %d: %s", nextProviderIndex, s.Providers[nextProviderIndex].Name())
			initialProviderIndex = nextProviderIndex // Reset the cycle detection start point
			s.mu.Unlock()

			attempt = 0 // Reset attempt count for the new provider cycle
			// Don't wait after switching, try the new provider immediately
			continue
		}

		// Wait before retrying (with the same provider)
		log.Printf("Waiting %dms before retrying batch with provider %s (attempt %d)", backoffMs, provider.Name(), attempt+1)
		select {
		case <-time.After(time.Duration(backoffMs) * time.Millisecond):
			attempt++ // Increment attempt count only if we waited and are retrying
			// Continue to the next attempt in the loop
		case <-ctx.Done():
			log.Printf("Context cancelled while waiting to retry batch with provider %s", provider.Name())
			return nil, fmt.Errorf("context cancelled while waiting to retry batch: %w", ctx.Err())
		}
	}
}

// --- Helper ---
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Handle potential multi-byte characters correctly if needed
	// For now, simple slicing:
	return s[:maxLen] + "..."
}
