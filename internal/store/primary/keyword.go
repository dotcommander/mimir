package primary

import (
	"mimir/internal/store"
)

// --- Keyword Search ---

// Ensure StoreImpl satisfies the KeywordSearcher interface
var _ store.KeywordSearcher = (*StoreImpl)(nil)
