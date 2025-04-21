package apihandlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AnswerRequest defines the expected JSON body for the /answer endpoint.
type AnswerRequest struct {
	Query string `json:"query" binding:"required"`
}

// AnswerResponse defines the JSON response for a successful answer generation.
type AnswerResponse struct {
	Answer string `json:"answer"`
}

// AnswerHandler handles requests to the /api/v1/answer endpoint.
func AnswerHandler(c *gin.Context) {
	var req AnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("ERROR binding answer request: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request body: %v", err)})
		return
	}

	// // Retrieve the App instance from the Gin context (Commented out as RAGService is disabled)
	// appInstanceAny, exists := c.Get("app")
	// if !exists {
	// 	log.Println("ERROR: App instance not found in Gin context")
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: App context not found"})
	// 	return
	// }
	//
	// appInstance, ok := appInstanceAny.(*app.App)
	// if !ok {
	// 	log.Println("ERROR: App instance in Gin context is not of type *app.App")
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: Invalid app context type"})
	// 	return
	// }

	// --- RAG Service Integration (Currently Disabled/Commented Out in App) ---
	// The RAGService field is commented out in internal/app/app.go.
	// To re-enable this, uncomment the RAGService field and its initialization in app.go
	// and ensure the services.RAGService type is defined.

	// // Check if RAG service is available
	// if appInstance.RAGService == nil {
	// 	log.Println("WARN: RAG service requested via API but not initialized/enabled.")
	// 	c.JSON(http.StatusNotImplemented, gin.H{"error": "RAG feature is not enabled or configured on the server"})
	// 	return
	// }
	// // Call the RAG service
	// answer, err := appInstance.RAGService.GenerateAnswer(c.Request.Context(), req.Query)
	// --- End RAG Service Integration ---

	// --- Temporary Error Response ---
	// Return an error indicating the feature is not implemented/enabled yet.
	c.JSON(http.StatusNotImplemented, gin.H{"error": "RAG functionality is currently disabled."})
	return
	// --- End Temporary Error Response ---
	/*
		if err != nil {
			log.Printf("ERROR generating RAG answer via API: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate answer: %v", err)})
			return
		}
		c.JSON(http.StatusOK, AnswerResponse{Answer: answer})
	*/
}
