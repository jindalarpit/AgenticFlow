package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/crypto"
	"github.com/agenticflow/agenticflow/server/internal/provider"
	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// onlineExecutionTimeout is the maximum duration for an online provider API call.
const onlineExecutionTimeout = 300 * time.Second

// OnlineExecutionEngine handles executing tasks against online AI providers.
// It decrypts credentials, builds the chat completion request, calls the
// provider adapter with streaming, forwards chunks via WebSocket, and updates
// the task status on completion or failure.
type OnlineExecutionEngine struct {
	q         db.Querier
	hub       *realtime.Hub
	encryptor *crypto.CredentialEncryptor
	registry  *provider.Registry
}

// NewOnlineExecutionEngine creates a new OnlineExecutionEngine with the given dependencies.
func NewOnlineExecutionEngine(q db.Querier, hub *realtime.Hub, encryptor *crypto.CredentialEncryptor, registry *provider.Registry) *OnlineExecutionEngine {
	return &OnlineExecutionEngine{
		q:         q,
		hub:       hub,
		encryptor: encryptor,
		registry:  registry,
	}
}

// Execute runs a task against an online provider. It validates the agent's
// configuration, decrypts provider credentials, builds the system prompt
// (agent instructions + deliverable type output_format), constructs a
// ChatCompletionRequest, calls the provider adapter with streaming, forwards
// each chunk via WebSocket, and updates the task as completed or failed.
//
// This method continues processing even if the client WebSocket disconnects.
// The provided context should NOT be tied to the client connection.
func (e *OnlineExecutionEngine) Execute(ctx context.Context, task db.Task, agent db.Agent) error {
	taskID := uuidToString(task.ID)

	// 1. Validate the agent has a model configured.
	if !agent.Model.Valid || strings.TrimSpace(agent.Model.String) == "" {
		errMsg := "no model configured for agent"
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 2. Validate the agent has a provider bound.
	if !agent.ProviderID.Valid {
		errMsg := "provider not found"
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 3. Get the provider from DB using agent.ProviderID and task.UserID.
	providerRecord, err := e.q.GetProvider(ctx, db.GetProviderParams{
		ID:     agent.ProviderID,
		UserID: task.UserID,
	})
	if err != nil {
		errMsg := "provider not found"
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("get provider: %w", err)
	}

	// 4. Check provider status is "active".
	if providerRecord.Status != "active" {
		errMsg := fmt.Sprintf("provider is unavailable: status is %s", providerRecord.Status)
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 5. Decrypt credentials.
	decryptedCreds, err := e.encryptor.Decrypt(providerRecord.CredentialsEncrypted)
	if err != nil {
		errMsg := "failed to decrypt provider credentials"
		slog.Error("online execution: decrypt credentials failed",
			"task_id", taskID, "provider_id", uuidToString(providerRecord.ID), "error", err)
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("decrypt credentials: %w", err)
	}

	// 6. Get the deliverable type to build the system prompt.
	outputFormat := ""
	if agent.DeliverableTypeID.Valid {
		dt, err := e.q.GetDeliverableType(ctx, db.GetDeliverableTypeParams{
			ID:     agent.DeliverableTypeID,
			UserID: task.UserID,
		})
		if err == nil {
			outputFormat = dt.OutputFormat
		} else {
			slog.Warn("online execution: failed to get deliverable type, proceeding without output_format",
				"task_id", taskID, "deliverable_type_id", uuidToString(agent.DeliverableTypeID), "error", err)
		}
	}

	// 7. Build system prompt: agent.Instructions + "\n" + outputFormat
	//    (omit separator if output_format is empty).
	systemPrompt := agent.Instructions
	if outputFormat != "" {
		if systemPrompt != "" {
			systemPrompt += "\n" + outputFormat
		} else {
			systemPrompt = outputFormat
		}
	}

	// 8. Build ChatCompletionRequest.
	messages := []provider.ChatMessage{}
	if systemPrompt != "" {
		messages = append(messages, provider.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, provider.ChatMessage{
		Role:    "user",
		Content: task.Prompt,
	})

	req := provider.ChatCompletionRequest{
		Model:    agent.Model.String,
		Messages: messages,
		Stream:   true,
	}

	// 9. Get the adapter for this provider type.
	adapter, ok := e.registry.Get(providerRecord.ProviderType)
	if !ok {
		errMsg := fmt.Sprintf("unsupported provider type: %s", providerRecord.ProviderType)
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// 10. Call adapter with a 300-second timeout.
	//     Use a detached context (not tied to client connection) with timeout.
	execCtx, cancel := context.WithTimeout(context.Background(), onlineExecutionTimeout)
	defer cancel()

	slog.Info("online execution: starting streaming",
		"task_id", taskID,
		"provider_type", providerRecord.ProviderType,
		"model", agent.Model.String)

	chunks, errCh := adapter.ChatCompletionStream(execCtx, json.RawMessage(decryptedCreds), req)

	// 11. Forward each chunk to WebSocket hub and aggregate content.
	var contentBuilder strings.Builder
	sequence := 0

	for chunk := range chunks {
		if chunk.Content != "" {
			sequence++
			contentBuilder.WriteString(chunk.Content)

			// Forward chunk via WebSocket as "task_output" event.
			// Continue processing even if WebSocket client disconnects.
			if e.hub != nil {
				e.hub.BroadcastTaskOutput(taskID, "stdout", chunk.Content, sequence)
			}
		}
	}

	// 12. Check for errors from the streaming.
	if streamErr := <-errCh; streamErr != nil {
		// Check if it was a timeout.
		if execCtx.Err() == context.DeadlineExceeded {
			errMsg := "provider request timed out after 300 seconds"
			e.failTask(ctx, task.ID, taskID, errMsg)
			return fmt.Errorf("%s", errMsg)
		}
		errMsg := fmt.Sprintf("provider returned error: %s", streamErr.Error())
		e.failTask(ctx, task.ID, taskID, errMsg)
		return fmt.Errorf("streaming error: %w", streamErr)
	}

	// 13. On completion: aggregate content, extract token usage, update task as completed.
	finalContent := contentBuilder.String()

	// If no streaming content was received, try a non-streaming call as fallback.
	if finalContent == "" {
		slog.Info("online execution: no streaming content, trying non-streaming call",
			"task_id", taskID)

		resp, err := adapter.ChatCompletion(execCtx, json.RawMessage(decryptedCreds), req)
		if err != nil {
			if execCtx.Err() == context.DeadlineExceeded {
				errMsg := "provider request timed out after 300 seconds"
				e.failTask(ctx, task.ID, taskID, errMsg)
				return fmt.Errorf("%s", errMsg)
			}
			errMsg := fmt.Sprintf("provider returned error: %s", err.Error())
			e.failTask(ctx, task.ID, taskID, errMsg)
			return fmt.Errorf("non-streaming error: %w", err)
		}

		finalContent = resp.Content

		// Broadcast the full response as a single task_output event.
		if e.hub != nil && finalContent != "" {
			e.hub.BroadcastTaskOutput(taskID, "stdout", finalContent, 1)
		}

		// Store token usage from non-streaming response.
		e.completeTask(ctx, task.ID, taskID, finalContent, &resp.Usage)
		return nil
	}

	// For streaming, token usage may not be available in chunks.
	// Complete the task with the aggregated content.
	e.completeTask(ctx, task.ID, taskID, finalContent, nil)
	return nil
}

// completeTask marks the task as completed with the given output and token usage.
func (e *OnlineExecutionEngine) completeTask(ctx context.Context, taskID pgtype.UUID, taskIDStr string, content string, usage *provider.TokenUsage) {
	// Build token usage JSON.
	var tokenUsageJSON []byte
	if usage != nil {
		tokenUsageJSON, _ = json.Marshal(usage)
	} else {
		// Default zero usage.
		tokenUsageJSON, _ = json.Marshal(provider.TokenUsage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		})
	}

	// Build output preview (first 500 chars).
	preview := content
	if len(preview) > 500 {
		preview = preview[:500]
	}

	err := e.q.UpdateTaskCompleted(ctx, db.UpdateTaskCompletedParams{
		ID:            taskID,
		ExitCode:      pgtype.Int4{Int32: 0, Valid: true},
		OutputPreview: pgtype.Text{String: preview, Valid: preview != ""},
		TokenUsage:    tokenUsageJSON,
	})
	if err != nil {
		slog.Error("online execution: failed to update task as completed",
			"task_id", taskIDStr, "error", err)
	}

	// Broadcast task_completed event.
	if e.hub != nil {
		e.hub.BroadcastTaskCompleted(taskIDStr, 0, "", content)
	}

	slog.Info("online execution: task completed", "task_id", taskIDStr, "content_length", len(content))
}

// failTask marks the task as failed with the given error message and broadcasts
// a task_failed event via WebSocket.
func (e *OnlineExecutionEngine) failTask(ctx context.Context, taskID pgtype.UUID, taskIDStr string, errMsg string) {
	err := e.q.UpdateTaskFailed(ctx, db.UpdateTaskFailedParams{
		ID:           taskID,
		ExitCode:     pgtype.Int4{Int32: 1, Valid: true},
		ErrorMessage: pgtype.Text{String: errMsg, Valid: true},
	})
	if err != nil {
		slog.Error("online execution: failed to update task as failed",
			"task_id", taskIDStr, "error", err)
	}

	// Broadcast task_failed event.
	if e.hub != nil {
		e.hub.BroadcastTaskFailed(taskIDStr, 1, errMsg, "")
	}

	slog.Warn("online execution: task failed", "task_id", taskIDStr, "error", errMsg)
}

// BuildChatCompletionRequest constructs a ChatCompletionRequest from the given
// agent instructions, task prompt, output format, and model. This is exported
// for testing purposes.
func BuildChatCompletionRequest(instructions, prompt, outputFormat, model string) provider.ChatCompletionRequest {
	systemPrompt := instructions
	if outputFormat != "" {
		if systemPrompt != "" {
			systemPrompt += "\n" + outputFormat
		} else {
			systemPrompt = outputFormat
		}
	}

	messages := []provider.ChatMessage{}
	if systemPrompt != "" {
		messages = append(messages, provider.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, provider.ChatMessage{
		Role:    "user",
		Content: prompt,
	})

	return provider.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	}
}

// ExtractTokenUsage extracts token usage from a provider response, defaulting
// missing fields to zero. This is exported for testing purposes.
func ExtractTokenUsage(usage *provider.TokenUsage) provider.TokenUsage {
	if usage == nil {
		return provider.TokenUsage{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		}
	}
	return *usage
}
