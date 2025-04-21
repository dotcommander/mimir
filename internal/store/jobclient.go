package store

import (
	"context"

	"encoding/json"
	"fmt"
	"log" // Add log import

	"github.com/google/uuid" // Add uuid import
	"github.com/hibiken/asynq"
	"mimir/internal/tasks" // Add tasks import for TypeEmbeddingJob
)

// AsynqJobClient is a concrete JobClient
// Enqueues embedding job tasks and records them to the JobStore
// Ensure it implements JobClient
var _ JobClient = (*AsynqJobClient)(nil)

type AsynqJobClient struct {
	client   *asynq.Client
	jobStore JobStore // Add JobStore dependency
}

func NewAsynqJobClient(redisAddr string, js JobStore) (*AsynqJobClient, error) {
	if js == nil {
		return nil, fmt.Errorf("JobStore cannot be nil for AsynqJobClient")
	}
	// Use Redis config from cfg if available, otherwise just address
	// For now, just using address and adding namespace
	// TODO: Pass full Redis config if needed for password/db
	cli := asynq.NewClient(asynq.RedisClientOpt{
		Addr: redisAddr,
		// Namespace: "mimir", // Removed: Not supported in this asynq version's RedisClientOpt
	})
	return &AsynqJobClient{client: cli, jobStore: js}, nil
}

func (jc *AsynqJobClient) Close() error {
	return jc.client.Close()
}

// Enqueue enqueues a task and records the event to the JobStore.
// It now accepts optional related entity information.
func (jc *AsynqJobClient) Enqueue(ctx context.Context, task *asynq.Task, relatedEntityType string, relatedEntityID int64, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	// Add nil check for the internal client
	if jc.client == nil {
		return nil, fmt.Errorf("AsynqJobClient internal client is not initialized")
	}
	// Add logging before the actual enqueue call
	log.Printf("DEBUG: Enqueuing task type '%s' via client: %p", task.Type(), jc.client) // Log client pointer address
	info, err := jc.client.EnqueueContext(ctx, task, opts...)
	if err != nil {
		// Log the error if the enqueue fails
		log.Printf("ERROR: Failed during jc.client.EnqueueContext for task type '%s': %v", task.Type(), err)
		// Return the error immediately
		return nil, err
	}
	// Log success info
	log.Printf("DEBUG: Successfully enqueued task type '%s', info: %+v", task.Type(), info)

	// Record the enqueue event to the database via JobStore
	jobUUID, err := uuid.Parse(info.ID)
	if err != nil {
		// Log the parsing error but don't fail the operation, as the job is already enqueued
		log.Printf("ERROR: Failed to parse Asynq Task ID '%s' to UUID: %v. Job record might be incomplete.", info.ID, err)
		// Optionally, you could try to record with a Nil UUID or skip recording
	}

	recordParams := JobRecordParams{
		JobID:             jobUUID, // Use parsed UUID
		TaskType:          task.Type(),
		Payload:           task.Payload(),
		Queue:             info.Queue,
		Status:            "enqueued", // Initial status
		RelatedEntityType: relatedEntityType,
		RelatedEntityID:   relatedEntityID,
	}
	if err := jc.jobStore.RecordJobEnqueue(ctx, recordParams); err != nil {
		// Log the recording error but don't fail the operation, as the job is already enqueued
		log.Printf("ERROR: Failed to record job enqueue event to DB for Task ID %s: %v", info.ID, err)
	}

	return info, nil // Return the info and nil error on success
}

func (jc *AsynqJobClient) EnqueueEmbeddingJob(ctx context.Context, contentID int64) error {
	payload := map[string]interface{}{"content_id": contentID}
	task := asynq.NewTask(tasks.TypeEmbeddingJob, encodePayload(payload)) // Use constant
	// Pass related entity info to the generic Enqueue method
	_, err := jc.Enqueue(ctx, task, "content", contentID, asynq.Queue("embeddings"))
	if err != nil {
		return fmt.Errorf("enqueue embedding job for content %d: %w", contentID, err)
	}
	return nil
}

func encodePayload(data map[string]interface{}) []byte {
	// naive JSON encode with no error handling for brevity
	b, _ := json.Marshal(data)
	return b
}

// Ensure AsynqJobClient still satisfies JobClient interface
var _ JobClient = (*AsynqJobClient)(nil)
