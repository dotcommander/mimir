package app

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai" // Add openai import
	"mimir/internal/config"             // Add config import
	"mimir/internal/inputprocessor"     // Add inputprocessor import
	"mimir/internal/costtracker"        // Add costtracker import
	"mimir/internal/services"           // Add services import
	"mimir/internal/store"
	// Removed import
	"mimir/internal/store/primary"
	"mimir/internal/store/vector" // Add vector import
	"mimir/pkg/categorizer"       // Add categorizer import
	// "github.com/hibiken/asynq" // No longer needed directly here
	log "github.com/sirupsen/logrus" // Use logrus
)

type App struct {
	// PrimaryStore     store.PrimaryStore // Removed - Use specific stores below
	JobClient        store.JobClient
	VectorStore      store.VectorStore
	CompletionService services.CompletionService // Interface for completion
	EmbeddingService store.EmbeddingService
	Config           *config.Config // Use the real config struct

	// Expose individual store interfaces for service initialization
	ContentStore       store.ContentStore
	TagStore           store.TagStore
	SourceStore        store.SourceStore
	CollectionStore    store.CollectionStore
	SearchHistoryStore store.SearchHistoryStore
	JobStore           store.JobStore          // Add JobStore field
	CostStore          store.CostTrackingStore // Add CostStore field
	CostTracker        costtracker.CostTracker // Add CostTracker field

	CategorizationService *services.CategorizationService // Add CategorizationService field

	// --- Initialized Services ---
	ContentService    *services.ContentService
	SourceService     *services.SourceService
	TagService        *services.TagService
	CollectionService *services.CollectionService
	SearchService     *services.SearchService
	BatchService      *services.BatchService    // Add BatchService field
	BatchAPIProvider  services.BatchAPIProvider // Add BatchAPIProvider field
	CostService       *services.CostService // Add CostService field
	// RAGService        *services.RAGService      // Commented out - undefined

	SummaryService services.SummaryService // Expose summary service for worker registration
}

func NewApp(cfg *config.Config, inputProc inputprocessor.Processor) (*App, error) {
	ctx := context.Background()
	app := &App{Config: cfg}

	if err := app.initPrimaryStore(ctx); err != nil {
		return nil, err
	}
	if err := app.initJobClient(); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initEmbeddingService(); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initCompletionService(); err != nil { // Initialize Completion Service
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initSummaryService(); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initBatchAPIProvider(); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initVectorStore(ctx); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initCategorizationService(); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	if err := app.initCoreServices(inputProc); err != nil {
		app.cleanupPartialInit()
		return nil, err
	}
	// if err := app.initRAGService(); err != nil { // Initialize RAG Service (after core services)
	// 	app.cleanupPartialInit()
	// 	return nil, err
	// }

	log.Println("Application initialization complete.")
	return app, nil
}

// --- Private Helper Methods ---

func (a *App) initPrimaryStore(ctx context.Context) error {
	ps, err := primary.NewPrimaryStore(ctx, a.Config.Database.Primary.DSN)
	if err != nil {
		return fmt.Errorf("init primary store: %w", err)
	}
	a.ContentStore = ps
	a.TagStore = ps
	a.SourceStore = ps
	a.CollectionStore = ps
	a.SearchHistoryStore = ps
	a.JobStore = ps
	a.CostStore = ps // StoreImpl implements CostTrackingStore
	a.CostTracker = costtracker.New() // Initialize the cost tracker service
	return nil
}

func (a *App) initJobClient() error {
	jc, err := store.NewAsynqJobClient(a.Config.Redis.Address, a.JobStore)
	if err != nil {
		return fmt.Errorf("init job client: %w", err)
	}
	a.JobClient = jc
	return nil
}

func (a *App) initEmbeddingService() error {
	var providers []services.EmbeddingProvider
	cfg := a.Config

	// Initialize OpenAI provider if enabled
	if cfg.Embedding.OpenaiApiKey != "" { // Check if API key is provided as indicator of enablement
		if cfg.Embedding.OpenaiApiKey == "" { // Redundant check, but keeps structure
			return fmt.Errorf("OpenAI API key is required but not set (embedding.providers.openai.api_key)")
		}
		// TODO: Pass CostTrackingStore and Pricing info from config to NewOpenAIProvider
		openaiProvider, err := services.NewOpenAIProvider(
			cfg.Embedding.OpenaiApiKey,
			cfg.Embedding.Model, // Assuming a single model for now, adjust if needed
			a.CostStore, // Pass CostStore
			cfg.Pricing["openai"], // Pass OpenAI pricing map
		)
		if err != nil {
			log.Printf("WARN: Failed to initialize OpenAI provider: %v", err)
			// Continue to try other providers
		} else if openaiProvider != nil {
			log.Printf("Initialized OpenAI embedding provider (Model: %s)", cfg.Embedding.Model) // Use top-level Model field
			providers = append(providers, openaiProvider)
		}
	}

	// TODO: Initialize Gemini provider if cfg.Embedding.Providers.Gemini.Enabled
	// geminiProvider, err := services.NewGeminiProvider(...)
	// if err == nil && geminiProvider != nil { providers = append(providers, geminiProvider) }

	// TODO: Initialize Anthropic provider if cfg.Embedding.Providers.Anthropic.Enabled
	// anthropicProvider, err := services.NewAnthropicProvider(...)
	// if err == nil && anthropicProvider != nil { providers = append(providers, anthropicProvider) }

	// TODO: Initialize Local provider if cfg.Embedding.Providers.Local.Enabled
	// localProvider, err := services.NewLocalProvider(...)
	// if err == nil && localProvider != nil { providers = append(providers, localProvider) }

	if len(providers) == 0 {
		log.Println("WARN: No embedding providers were successfully initialized. Embedding generation will fail.")
		// Optionally return an error if no providers are essential
		// return fmt.Errorf("no embedding providers configured or initialized successfully")
	}

	retryStrategy := &services.SimpleRetryStrategy{MaxAttempts: 3, BaseDelayMs: 200}
	embeddingService, err := services.NewFallbackEmbeddingService(providers, retryStrategy)
	if err != nil {
		return fmt.Errorf("init embedding service: %w", err)
	}
	a.EmbeddingService = embeddingService
	return nil
}

func (a *App) initCompletionService() error {
	cfg := a.Config
	if !cfg.RAG.Enabled {
		log.Println("RAG is disabled, skipping CompletionService initialization.")
		return nil // Not an error if RAG is disabled
	}

	var completer services.CompletionService
	var err error

	switch cfg.RAG.Provider {
	case "gemini":
		// Assuming GeminiProvider's New function signature is updated
		// Note: Using Embedding API Key and Model Name for now. Consider separate config if needed.
		completer, err = services.NewGeminiProvider( // Call with correct args
			cfg.Embedding.GoogleApiKey, // Reuse embedding key for now
			cfg.Embedding.GeminiModelName, // Embedding model // TODO: This field doesn't exist in current config (config.go:31), needs update in config or here
		)
		// The following arguments are not expected by NewGeminiProvider:
		// cfg.RAG.Model, a.CostStore, cfg.Pricing

		if err != nil {
			return fmt.Errorf("failed to initialize Gemini completion provider: %w", err)
		}
	// case "openai": // Add OpenAI completion provider initialization here if needed
	default:
		return fmt.Errorf("unknown or unsupported RAG provider configured: %s", cfg.RAG.Provider)
	}
	a.CompletionService = completer
	return nil
}

func (a *App) initSummaryService() error {
	cfg := a.Config
	if cfg.Summarization.Enabled {
		switch cfg.Summarization.Provider {
		case "openai":
			// Load prompt using the new helper function
			promptContent, err := config.LoadPromptContent(cfg.Summarization.Prompt, "summarize.txt")
			if err != nil {
				log.Warnf("Failed to load summarization prompt: %v. Summarization might not work correctly.", err)
				// Optionally return err here if prompt is mandatory: return fmt.Errorf("load summarization prompt: %w", err)
				promptContent = "" // Fallback to empty prompt
			}
			// Pass pricing info to the summary service constructor
			a.SummaryService = services.NewOpenAISummaryService(
				cfg.Embedding.OpenaiApiKey,
				cfg.Summarization.Model,
				promptContent,
				a.CostStore, // Pass CostStore
				cfg.Pricing["openai"], // Pass OpenAI pricing map
			)
		default:
			log.Warnf("Unsupported summarization provider: %s", cfg.Summarization.Provider)
			a.SummaryService = services.NewNoopSummaryService()
		}
	} else {
		a.SummaryService = services.NewNoopSummaryService()
	}
	return nil
}

func (a *App) initBatchAPIProvider() error {
	// Pass CostStore and Pricing info
	batchAPIProvider, err := services.NewOpenAIBatchProvider(
		a.Config.Embedding.OpenaiApiKey,
		a.CostStore,
		a.Config.Pricing["openai"], // Pass config.PricingInfo map
	)
	if err != nil {
		return fmt.Errorf("init OpenAI Batch API provider: %w", err)
	}
	a.BatchAPIProvider = batchAPIProvider
	return nil
}

func (a *App) initVectorStore(ctx context.Context) error {
	cfg := a.Config
	if cfg.Database.Vector.DSN == "" {
		return fmt.Errorf("vector store DSN (Database.Vector.DSN) is required but not configured")
	}
	vectorStore, err := vector.NewStore(ctx, cfg.Database.Vector.DSN)
	if err != nil {
		return fmt.Errorf("init postgres vector store: %w", err)
	}
	a.VectorStore = vectorStore
	return nil
}

func (a *App) initCategorizationService() error {
	cfg := a.Config
	var contentCategorizer categorizer.ContentCategorizer
	if cfg.Categorization.Type == "llm" {
		switch cfg.Categorization.Provider {
		case "openai":
			if cfg.Embedding.OpenaiApiKey == "" {
				return fmt.Errorf("OpenAI API key is required for categorization but not set")
			}
			openaiClient := openai.NewClient(cfg.Embedding.OpenaiApiKey)
			// Load prompt using the new helper function
			promptContent, err := config.LoadPromptContent(cfg.Categorization.PromptTemplate, "categorize.txt")
			if err != nil {
				// Log the error and potentially disable categorization or return the error
				log.Warnf("Failed to load categorization prompt: %v. LLM Categorization might not work correctly.", err)
				// Optionally return err here if prompt is mandatory: return fmt.Errorf("load categorization prompt: %w", err)
				// For now, we allow it to proceed with an empty prompt (which will likely fail later)
				promptContent = "" // Set to empty string to avoid nil pointer issues downstream if needed
			}
			// Pass pricing info to the categorizer constructor
			contentCategorizer = categorizer.NewLLMCategorizer(
				openaiClient, cfg.Categorization.Model, promptContent,
				a.CostTracker, cfg.Pricing["openai"], // Pass CostTracker service
			)
		default:
			log.Warnf("WARN: Unsupported LLM categorization provider '%s'. Categorization disabled.", cfg.Categorization.Provider)
		}
	}
	tagService := services.NewTagService(a.TagStore)
	collectionService := services.NewCollectionService(a.CollectionStore, a.ContentStore, a.TagStore)
	a.CategorizationService = services.NewCategorizationService(contentCategorizer, tagService, collectionService, a.ContentStore)
	return nil
}

func (a *App) initCoreServices(inputProc inputprocessor.Processor) error {
	cfg := a.Config
	a.SourceService = services.NewSourceService(a.SourceStore)
	a.TagService = services.NewTagService(a.TagStore)
	a.CollectionService = services.NewCollectionService(a.CollectionStore, a.ContentStore, a.TagStore)
	a.ContentService = services.NewContentService(services.ContentServiceDeps{
		ContentStore:          a.ContentStore,
		TagStore:              a.TagStore,
		JobClient:             a.JobClient,
		SourceService:         a.SourceService,
		Processor:             inputProc,
		SummaryService:        a.SummaryService,
		TaggingService:        services.NewNoopTaggingService(),
		CategorizationService: a.CategorizationService,
		Config:                cfg,
	})
	// Need the concrete primary store that implements KeywordSearcher
	ps, ok := a.ContentStore.(*primary.StoreImpl) // Type assertion for KeywordSearcher
	if !ok {
		// This should not happen if initPrimaryStore worked correctly
		return fmt.Errorf("internal error: ContentStore is not of expected type *primary.StoreImpl")
	}
	// Pass the concrete store for both ContentStore and KeywordSearcher interfaces
	a.SearchService = services.NewSearchService(ps, ps, a.VectorStore, a.EmbeddingService, a.SearchHistoryStore)
	a.BatchService = services.NewBatchService(a.JobStore)
	a.CostService = services.NewCostService(a.CostStore) // Initialize CostService
	return nil
}

func (a *App) initRAGService() error {
	cfg := a.Config
	if !cfg.RAG.Enabled {
		log.Println("RAG is disabled, skipping RAGService initialization.")
		return nil
	}
	if a.CompletionService == nil {
		log.Warnln("RAG is enabled but CompletionService is not initialized (check provider config/errors). RAGService will not be available.")
		return nil // Not a fatal error, but RAG won't work
	}

	// ragService, err := services.NewRAGService( // Keep commented out due to potential undefined: services.RAGService
	// 	*a.SearchService,         // Pass initialized SearchService
	// 	a.CompletionService,      // Pass initialized CompletionService
	// 	config.LoadPromptContent, // Pass the prompt loader function
	// 	cfg.RAG,                  // Pass the RAG config section
	// )
	// a.RAGService = ragService // Assign even if err != nil, caller checks RAGService field
	// return err // Return the error from NewRAGService
	return nil // Add missing return
	return nil // Add missing return
}

func (a *App) cleanupPartialInit() {
	if a.JobClient != nil {
		a.JobClient.Close()
	}
	if a.VectorStore != nil {
		a.VectorStore.Close()
	}
	// Cannot call Close on a.ContentStore (interface)
	// Add cleanup for CompletionService if it has a Close method
	if cs, ok := a.CompletionService.(interface{ Close() error }); ok && cs != nil {
		if err := cs.Close(); err != nil {
			log.Printf("Error closing CompletionService: %v", err)
		}
	}
}
