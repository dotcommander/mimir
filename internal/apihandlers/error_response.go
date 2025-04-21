package apihandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError defines standard error response
// Example: { "error": { "code": "bad_request", "message": "Invalid ID" } }
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error APIError `json:"error"`
}

// JSONError sends a structured error response
func JSONError(ctx *gin.Context, status int, code, msg string) {
	ctx.JSON(status, errorResponse{Error: APIError{Code: code, Message: msg}})
}

// Convenience wrappers
func BadRequest(ctx *gin.Context, msg string) {
	JSONError(ctx, http.StatusBadRequest, "bad_request", msg)
}

func NotFound(ctx *gin.Context, msg string) {
	JSONError(ctx, http.StatusNotFound, "not_found", msg)
}

func Internal(ctx *gin.Context, msg string) {
	JSONError(ctx, http.StatusInternalServerError, "internal_error", msg)
}

func Conflict(ctx *gin.Context, msg string) {
	JSONError(ctx, http.StatusConflict, "conflict", msg)
}
