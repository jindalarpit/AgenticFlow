package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agenticflow/agenticflow/server/internal/realtime"
	db "github.com/agenticflow/agenticflow/server/pkg/db/generated"
)

// Validation constants for agent fields.
const (
	maxAgentNameLength         = 64
	maxAgentDescriptionLength  = 255
	maxAgentInstructionsLength = 50000
	maxAgentModelLength        = 100
	maxCustomEnvPairs          = 20
	maxCustomEnvKeyLength      = 64
	maxCustomEnvValueLength    = 1024
	maxCustomArgs              = 20
	maxCustomArgLength         = 256
	minConcurrentTasks         = 1
	maxConcurrentTasks         = 20
)

// agentNameRegex validates agent names: starts with alphanumeric,
// followed by alphanumeric, hyphens, or underscores. 1-64 chars total.
var agentNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// AgentService encapsulates business logic for agent CRUD operations,
// runtime binding validation, and real-time event broadcasting.
type AgentService struct {
	q   db.Querier
	hub *realtime.Hub
}

// NewAgentService creates a new AgentService.
func NewAgentService(q db.Querier, hub *realtime.Hub) *AgentService {
	return &AgentService{q: q, hub: hub}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

// CreateAgentParams holds the validated parameters for creating an agent.
type CreateAgentParams struct {
	UserID             string
	Name               string
	Description        string
	Instructions       string
	AvatarURL          *string
	RuntimeID          string
	CustomEnv          map[string]string
	CustomArgs         []string
	Model              string
	Visibility         string
	MaxConcurrentTasks int32
	MCPConfig          json.RawMessage
	RuntimeMode        string
	ProviderID         string
	DeliverableTypeID  string
}

// UpdateAgentParams holds the parameters for updating an agent.
// All fields are pointers; nil means "do not change".
type UpdateAgentParams struct {
	UserID             string
	AgentID            string
	Name               *string
	Description        *string
	Instructions       *string
	AvatarURL          *string
	RuntimeID          *string
	CustomEnv          *map[string]string
	CustomArgs         *[]string
	Model              *string
	Visibility         *string
	MaxConcurrentTasks *int32
	MCPConfig          json.RawMessage
	MCPConfigPresent   bool // true if mcp_config was explicitly in the request
	RuntimeMode        *string
	ProviderID         *string
	DeliverableTypeID  *string
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// Create validates all fields, verifies the runtime exists, rejects duplicate
// names per user, and creates the agent. Returns the created agent or a typed
// ServiceError.
func (s *AgentService) Create(ctx context.Context, params CreateAgentParams) (db.Agent, *ServiceError) {
	// --- Validate name ---
	if params.Name == "" {
		return db.Agent{}, Validation("name is required")
	}
	if !agentNameRegex.MatchString(params.Name) {
		return db.Agent{}, Validation("name must start with an alphanumeric character and contain only alphanumeric characters, hyphens, and underscores (1-64 characters)")
	}

	// --- Validate description ---
	if utf8.RuneCountInString(params.Description) > maxAgentDescriptionLength {
		return db.Agent{}, Validation(fmt.Sprintf("description must be %d characters or fewer", maxAgentDescriptionLength))
	}

	// --- Validate instructions ---
	if utf8.RuneCountInString(params.Instructions) > maxAgentInstructionsLength {
		return db.Agent{}, Validation(fmt.Sprintf("instructions must be %d characters or fewer", maxAgentInstructionsLength))
	}

	// --- Validate model ---
	if utf8.RuneCountInString(params.Model) > maxAgentModelLength {
		return db.Agent{}, Validation(fmt.Sprintf("model must be %d characters or fewer", maxAgentModelLength))
	}

	// --- Validate custom_env ---
	if len(params.CustomEnv) > maxCustomEnvPairs {
		return db.Agent{}, Validation(fmt.Sprintf("custom_env must have %d or fewer key-value pairs", maxCustomEnvPairs))
	}
	for key, value := range params.CustomEnv {
		keyLen := utf8.RuneCountInString(key)
		if keyLen < 1 || keyLen > maxCustomEnvKeyLength {
			return db.Agent{}, Validation(fmt.Sprintf("custom_env key must be between 1 and %d characters", maxCustomEnvKeyLength))
		}
		valueLen := utf8.RuneCountInString(value)
		if valueLen < 1 || valueLen > maxCustomEnvValueLength {
			return db.Agent{}, Validation(fmt.Sprintf("custom_env value must be between 1 and %d characters", maxCustomEnvValueLength))
		}
	}

	// --- Validate custom_args ---
	if len(params.CustomArgs) > maxCustomArgs {
		return db.Agent{}, Validation(fmt.Sprintf("custom_args must have %d or fewer items", maxCustomArgs))
	}
	for _, arg := range params.CustomArgs {
		if utf8.RuneCountInString(arg) > maxCustomArgLength {
			return db.Agent{}, Validation(fmt.Sprintf("each custom_args item must be %d characters or fewer", maxCustomArgLength))
		}
	}

	// --- Validate max_concurrent_tasks ---
	if params.MaxConcurrentTasks == 0 {
		params.MaxConcurrentTasks = 1 // default
	}
	if params.MaxConcurrentTasks < minConcurrentTasks || params.MaxConcurrentTasks > maxConcurrentTasks {
		return db.Agent{}, Validation(fmt.Sprintf("max_concurrent_tasks must be between %d and %d", minConcurrentTasks, maxConcurrentTasks))
	}

	// --- Validate visibility ---
	if params.Visibility == "" {
		params.Visibility = "private" // default
	}
	if params.Visibility != "private" && params.Visibility != "shared" {
		return db.Agent{}, Validation("visibility must be \"private\" or \"shared\"")
	}

	// --- Validate mcp_config (if provided) ---
	var mcpConfigBytes []byte
	if len(params.MCPConfig) > 0 && string(params.MCPConfig) != "null" {
		if !json.Valid(params.MCPConfig) {
			return db.Agent{}, Validation("mcp_config must be valid JSON")
		}
		mcpConfigBytes = []byte(params.MCPConfig)
	}

	// --- Default runtime_mode ---
	if params.RuntimeMode == "" {
		params.RuntimeMode = "local"
	}
	if params.RuntimeMode != "local" && params.RuntimeMode != "online" {
		return db.Agent{}, Validation("runtime_mode must be \"local\" or \"online\"")
	}

	// --- Reject both provider_id and runtime_id set simultaneously ---
	if params.ProviderID != "" && params.RuntimeID != "" {
		return db.Agent{}, Unprocessable("only one of provider_id or runtime_id may be specified")
	}

	// --- Parse user UUID ---
	userUUID, err := parseUUID(params.UserID)
	if err != nil {
		return db.Agent{}, Internal("invalid user id")
	}

	// --- Runtime mode specific validation ---
	var runtimeUUID pgtype.UUID
	var providerUUID pgtype.UUID

	switch params.RuntimeMode {
	case "local":
		// Local mode: require runtime_id, reject provider_id
		if params.RuntimeID == "" {
			return db.Agent{}, Validation("runtime_id is required")
		}
		runtimeUUID, err = parseUUID(params.RuntimeID)
		if err != nil {
			return db.Agent{}, Validation("invalid runtime_id format")
		}
		_, err = s.q.GetRuntimeByID(ctx, runtimeUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Agent{}, Validation("runtime_id does not reference an existing runtime")
			}
			slog.Error("create agent: get runtime failed", "runtime_id", params.RuntimeID, "error", err)
			return db.Agent{}, Internal("failed to verify runtime")
		}

	case "online":
		// Online mode: require provider_id, reject runtime_id
		if params.ProviderID == "" {
			return db.Agent{}, Unprocessable("provider_id is required for online agents")
		}
		providerUUID, err = parseUUID(params.ProviderID)
		if err != nil {
			return db.Agent{}, Validation("invalid provider_id format")
		}
		// Verify provider exists, belongs to user, and is active
		provider, err := s.q.GetProvider(ctx, db.GetProviderParams{
			ID:     providerUUID,
			UserID: userUUID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Agent{}, Unprocessable("provider is invalid or inactive")
			}
			slog.Error("create agent: get provider failed", "provider_id", params.ProviderID, "error", err)
			return db.Agent{}, Internal("failed to verify provider")
		}
		if provider.Status != "active" {
			return db.Agent{}, Unprocessable("provider is invalid or inactive")
		}

		// Online mode: require model (non-empty, ≤100 chars)
		if params.Model == "" {
			return db.Agent{}, Unprocessable("model is required for online agents")
		}
		if utf8.RuneCountInString(params.Model) > maxAgentModelLength {
			return db.Agent{}, Validation(fmt.Sprintf("model must be %d characters or fewer", maxAgentModelLength))
		}

		// Validate model is in provider's models list
		var providerModels []string
		if err := json.Unmarshal(provider.Models, &providerModels); err != nil {
			slog.Error("create agent: unmarshal provider models failed", "provider_id", params.ProviderID, "error", err)
			return db.Agent{}, Internal("failed to read provider models")
		}
		modelFound := false
		for _, m := range providerModels {
			if m == params.Model {
				modelFound = true
				break
			}
		}
		if !modelFound {
			return db.Agent{}, Unprocessable("model is not available on the selected provider")
		}
	}

	// --- Default deliverable_type_id based on runtime_mode ---
	var deliverableTypeUUID pgtype.UUID
	if params.DeliverableTypeID != "" {
		deliverableTypeUUID, err = parseUUID(params.DeliverableTypeID)
		if err != nil {
			return db.Agent{}, Validation("invalid deliverable_type_id format")
		}
	} else {
		// Apply default based on runtime_mode
		var defaultName string
		if params.RuntimeMode == "local" {
			defaultName = "Code Execution"
		} else {
			defaultName = "Chat Completion"
		}
		dt, err := s.q.GetSystemDeliverableTypeByName(ctx, defaultName)
		if err != nil {
			slog.Error("create agent: get default deliverable type failed", "name", defaultName, "error", err)
			return db.Agent{}, Internal("failed to resolve default deliverable type")
		}
		deliverableTypeUUID = dt.ID
	}

	// --- Reject Code Execution + online incompatibility ---
	if params.RuntimeMode == "online" && deliverableTypeUUID.Valid {
		dt, err := s.q.GetDeliverableType(ctx, db.GetDeliverableTypeParams{
			ID:     deliverableTypeUUID,
			UserID: userUUID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Agent{}, Unprocessable("deliverable type not found")
			}
			slog.Error("create agent: get deliverable type failed", "error", err)
			return db.Agent{}, Internal("failed to verify deliverable type")
		}
		if dt.Name == "Code Execution" {
			return db.Agent{}, Unprocessable("online agents cannot use the Code Execution deliverable type")
		}
	}

	// --- Reject duplicate names per user ---
	_, err = s.q.GetAgentByName(ctx, db.GetAgentByNameParams{
		UserID: userUUID,
		Name:   params.Name,
	})
	if err == nil {
		// Agent with this name already exists for this user.
		return db.Agent{}, Conflict("an agent with this name already exists")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("create agent: check duplicate name failed", "error", err)
		return db.Agent{}, Internal("failed to check agent name")
	}

	// --- Marshal JSON fields ---
	customEnvJSON, err := json.Marshal(params.CustomEnv)
	if err != nil {
		customEnvJSON = []byte("{}")
	}
	if params.CustomEnv == nil {
		customEnvJSON = []byte("{}")
	}

	customArgsJSON, err := json.Marshal(params.CustomArgs)
	if err != nil {
		customArgsJSON = []byte("[]")
	}
	if params.CustomArgs == nil {
		customArgsJSON = []byte("[]")
	}

	// --- Create agent ---
	agent, err := s.q.CreateAgent(ctx, db.CreateAgentParams{
		UserID:             userUUID,
		Name:               params.Name,
		Description:        params.Description,
		Instructions:       params.Instructions,
		RuntimeID:          runtimeUUID,
		Model:              pgtype.Text{String: params.Model, Valid: params.Model != ""},
		CustomEnv:          customEnvJSON,
		CustomArgs:         customArgsJSON,
		MaxConcurrentTasks: params.MaxConcurrentTasks,
		Visibility:         params.Visibility,
		AvatarUrl:          pgtype.Text{String: ptrToString(params.AvatarURL), Valid: params.AvatarURL != nil},
		McpConfig:          mcpConfigBytes,
		RuntimeMode:        params.RuntimeMode,
		ProviderID:         providerUUID,
		DeliverableTypeID:  deliverableTypeUUID,
	})
	if err != nil {
		slog.Error("create agent: insert failed", "user_id", params.UserID, "error", err)
		return db.Agent{}, Internal("failed to create agent")
	}

	slog.Info("agent created", "agent_id", uuidToString(agent.ID), "name", agent.Name, "user_id", params.UserID)

	// Broadcast agent_created event via WebSocket.
	if s.hub != nil {
		s.hub.BroadcastAgentCreated(agent)
	}

	return agent, nil
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// List returns all agents visible to the authenticated user (owned + shared),
// sorted by created_at descending.
func (s *AgentService) List(ctx context.Context, userID string) ([]db.Agent, *ServiceError) {
	userUUID, err := parseUUID(userID)
	if err != nil {
		return nil, Internal("invalid user id")
	}

	agents, err := s.q.ListAgentsByUser(ctx, userUUID)
	if err != nil {
		slog.Error("list agents: query failed", "user_id", userID, "error", err)
		return nil, Internal("failed to list agents")
	}

	return agents, nil
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// Get retrieves a single agent by ID. Returns NotFound if the agent does not
// exist.
func (s *AgentService) Get(ctx context.Context, agentID string) (db.Agent, *ServiceError) {
	agentUUID, err := parseUUID(agentID)
	if err != nil {
		return db.Agent{}, Validation("invalid agent id format")
	}

	agent, err := s.q.GetAgent(ctx, agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Agent{}, NotFound("agent not found")
		}
		slog.Error("get agent: query failed", "agent_id", agentID, "error", err)
		return db.Agent{}, Internal("failed to get agent")
	}

	return agent, nil
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update validates changed fields, persists the update, and broadcasts an
// agent_updated event via WebSocket. Returns the updated agent or a typed
// ServiceError.
func (s *AgentService) Update(ctx context.Context, params UpdateAgentParams) (db.Agent, *ServiceError) {
	agentUUID, err := parseUUID(params.AgentID)
	if err != nil {
		return db.Agent{}, Validation("invalid agent id format")
	}

	userUUID, err := parseUUID(params.UserID)
	if err != nil {
		return db.Agent{}, Internal("invalid user id")
	}

	// --- Validate name (if provided) ---
	if params.Name != nil {
		if *params.Name == "" {
			return db.Agent{}, Validation("name is required")
		}
		if !agentNameRegex.MatchString(*params.Name) {
			return db.Agent{}, Validation("name must start with an alphanumeric character and contain only alphanumeric characters, hyphens, and underscores (1-64 characters)")
		}
	}

	// --- Validate description (if provided) ---
	if params.Description != nil {
		if utf8.RuneCountInString(*params.Description) > maxAgentDescriptionLength {
			return db.Agent{}, Validation(fmt.Sprintf("description must be %d characters or fewer", maxAgentDescriptionLength))
		}
	}

	// --- Validate instructions (if provided) ---
	if params.Instructions != nil {
		if utf8.RuneCountInString(*params.Instructions) > maxAgentInstructionsLength {
			return db.Agent{}, Validation(fmt.Sprintf("instructions must be %d characters or fewer", maxAgentInstructionsLength))
		}
	}

	// --- Validate model (if provided) ---
	if params.Model != nil {
		if utf8.RuneCountInString(*params.Model) > maxAgentModelLength {
			return db.Agent{}, Validation(fmt.Sprintf("model must be %d characters or fewer", maxAgentModelLength))
		}
	}

	// --- Validate custom_env (if provided) ---
	if params.CustomEnv != nil {
		if len(*params.CustomEnv) > maxCustomEnvPairs {
			return db.Agent{}, Validation(fmt.Sprintf("custom_env must have %d or fewer key-value pairs", maxCustomEnvPairs))
		}
		for key, value := range *params.CustomEnv {
			keyLen := utf8.RuneCountInString(key)
			if keyLen < 1 || keyLen > maxCustomEnvKeyLength {
				return db.Agent{}, Validation(fmt.Sprintf("custom_env key must be between 1 and %d characters", maxCustomEnvKeyLength))
			}
			valueLen := utf8.RuneCountInString(value)
			if valueLen < 1 || valueLen > maxCustomEnvValueLength {
				return db.Agent{}, Validation(fmt.Sprintf("custom_env value must be between 1 and %d characters", maxCustomEnvValueLength))
			}
		}
	}

	// --- Validate custom_args (if provided) ---
	if params.CustomArgs != nil {
		if len(*params.CustomArgs) > maxCustomArgs {
			return db.Agent{}, Validation(fmt.Sprintf("custom_args must have %d or fewer items", maxCustomArgs))
		}
		for _, arg := range *params.CustomArgs {
			if utf8.RuneCountInString(arg) > maxCustomArgLength {
				return db.Agent{}, Validation(fmt.Sprintf("each custom_args item must be %d characters or fewer", maxCustomArgLength))
			}
		}
	}

	// --- Validate max_concurrent_tasks (if provided) ---
	if params.MaxConcurrentTasks != nil {
		if *params.MaxConcurrentTasks < minConcurrentTasks || *params.MaxConcurrentTasks > maxConcurrentTasks {
			return db.Agent{}, Validation(fmt.Sprintf("max_concurrent_tasks must be between %d and %d", minConcurrentTasks, maxConcurrentTasks))
		}
	}

	// --- Validate visibility (if provided) ---
	if params.Visibility != nil {
		if *params.Visibility != "private" && *params.Visibility != "shared" {
			return db.Agent{}, Validation("visibility must be \"private\" or \"shared\"")
		}
	}

	// --- Validate runtime_id (if provided) — runtime binding validation ---
	var runtimeUUID pgtype.UUID
	if params.RuntimeID != nil {
		if *params.RuntimeID == "" {
			return db.Agent{}, Validation("runtime_id cannot be empty")
		}
		runtimeUUID, err = parseUUID(*params.RuntimeID)
		if err != nil {
			return db.Agent{}, Validation("invalid runtime_id format")
		}
		_, err = s.q.GetRuntimeByID(ctx, runtimeUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return db.Agent{}, Validation("runtime_id does not reference an existing runtime")
			}
			slog.Error("update agent: get runtime failed", "runtime_id", *params.RuntimeID, "error", err)
			return db.Agent{}, Internal("failed to verify runtime")
		}
	}

	// --- Validate runtime_mode (if provided) ---
	if params.RuntimeMode != nil {
		if *params.RuntimeMode != "local" && *params.RuntimeMode != "online" {
			return db.Agent{}, Validation("runtime_mode must be \"local\" or \"online\"")
		}
	}

	// --- Reject both provider_id and runtime_id set simultaneously ---
	if params.ProviderID != nil && params.RuntimeID != nil {
		return db.Agent{}, Unprocessable("only one of provider_id or runtime_id may be specified")
	}

	// --- Fetch existing agent to determine effective runtime_mode ---
	existingAgent, err := s.q.GetAgent(ctx, agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Agent{}, NotFound("agent not found")
		}
		slog.Error("update agent: get existing agent failed", "agent_id", params.AgentID, "error", err)
		return db.Agent{}, Internal("failed to get agent")
	}

	// Determine effective runtime_mode after update
	effectiveMode := existingAgent.RuntimeMode
	if params.RuntimeMode != nil {
		effectiveMode = *params.RuntimeMode
	}

	// Determine effective model after update
	effectiveModel := existingAgent.Model.String
	if params.Model != nil {
		effectiveModel = *params.Model
	}

	// --- Validate provider_id (if provided or if switching to online mode) ---
	var providerUUID pgtype.UUID
	var setProviderID bool
	if params.ProviderID != nil {
		setProviderID = true
		if *params.ProviderID == "" {
			// Explicitly clearing provider_id (e.g., switching to local)
			providerUUID = pgtype.UUID{}
		} else {
			providerUUID, err = parseUUID(*params.ProviderID)
			if err != nil {
				return db.Agent{}, Validation("invalid provider_id format")
			}
			// Verify provider exists, belongs to user, and is active
			provider, err := s.q.GetProvider(ctx, db.GetProviderParams{
				ID:     providerUUID,
				UserID: userUUID,
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return db.Agent{}, Unprocessable("provider is invalid or inactive")
				}
				slog.Error("update agent: get provider failed", "provider_id", *params.ProviderID, "error", err)
				return db.Agent{}, Internal("failed to verify provider")
			}
			if provider.Status != "active" {
				return db.Agent{}, Unprocessable("provider is invalid or inactive")
			}

			// Validate model is in provider's models list
			if effectiveMode == "online" && effectiveModel != "" {
				var providerModels []string
				if err := json.Unmarshal(provider.Models, &providerModels); err != nil {
					slog.Error("update agent: unmarshal provider models failed", "provider_id", *params.ProviderID, "error", err)
					return db.Agent{}, Internal("failed to read provider models")
				}
				modelFound := false
				for _, m := range providerModels {
					if m == effectiveModel {
						modelFound = true
						break
					}
				}
				if !modelFound {
					return db.Agent{}, Unprocessable("model is not available on the selected provider")
				}
			}
		}
	}

	// --- Online mode validations ---
	if effectiveMode == "online" {
		// If switching to online, require provider_id
		effectiveProviderID := existingAgent.ProviderID
		if setProviderID {
			effectiveProviderID = providerUUID
		}
		if !effectiveProviderID.Valid {
			return db.Agent{}, Unprocessable("provider_id is required for online agents")
		}

		// Online mode requires model
		if effectiveModel == "" {
			return db.Agent{}, Unprocessable("model is required for online agents")
		}
		if utf8.RuneCountInString(effectiveModel) > maxAgentModelLength {
			return db.Agent{}, Validation(fmt.Sprintf("model must be %d characters or fewer", maxAgentModelLength))
		}

		// If model is changing and provider is not changing, validate against existing provider
		if params.Model != nil && !setProviderID && effectiveProviderID.Valid {
			provider, err := s.q.GetProvider(ctx, db.GetProviderParams{
				ID:     effectiveProviderID,
				UserID: userUUID,
			})
			if err == nil {
				var providerModels []string
				if err := json.Unmarshal(provider.Models, &providerModels); err == nil {
					modelFound := false
					for _, m := range providerModels {
						if m == effectiveModel {
							modelFound = true
							break
						}
					}
					if !modelFound {
						return db.Agent{}, Unprocessable("model is not available on the selected provider")
					}
				}
			}
		}
	}

	// --- Local mode validations ---
	if effectiveMode == "local" {
		// If switching to local, require runtime_id
		effectiveRuntimeID := existingAgent.RuntimeID
		if params.RuntimeID != nil {
			effectiveRuntimeID = runtimeUUID
		}
		if !effectiveRuntimeID.Valid {
			return db.Agent{}, Validation("runtime_id is required for local agents")
		}
	}

	// --- Handle deliverable_type_id ---
	var deliverableTypeUUID pgtype.UUID
	var setDeliverableTypeID bool
	if params.DeliverableTypeID != nil {
		setDeliverableTypeID = true
		if *params.DeliverableTypeID == "" {
			deliverableTypeUUID = pgtype.UUID{}
		} else {
			deliverableTypeUUID, err = parseUUID(*params.DeliverableTypeID)
			if err != nil {
				return db.Agent{}, Validation("invalid deliverable_type_id format")
			}
		}
	}

	// --- Build update params (COALESCE-based: nil = keep existing) ---
	dbParams := db.UpdateAgentParams{
		ID:     agentUUID,
		UserID: userUUID,
	}

	if params.Name != nil {
		dbParams.Name = pgtype.Text{String: *params.Name, Valid: true}
	}
	if params.Description != nil {
		dbParams.Description = pgtype.Text{String: *params.Description, Valid: true}
	}
	if params.Instructions != nil {
		dbParams.Instructions = pgtype.Text{String: *params.Instructions, Valid: true}
	}
	if params.RuntimeID != nil {
		dbParams.SetRuntimeID = true
		dbParams.RuntimeID = runtimeUUID
	}
	if params.Model != nil {
		dbParams.Model = pgtype.Text{String: *params.Model, Valid: true}
	}
	if params.CustomEnv != nil {
		envJSON, err := json.Marshal(*params.CustomEnv)
		if err != nil {
			envJSON = []byte("{}")
		}
		dbParams.CustomEnv = envJSON
	}
	if params.CustomArgs != nil {
		argsJSON, err := json.Marshal(*params.CustomArgs)
		if err != nil {
			argsJSON = []byte("[]")
		}
		dbParams.CustomArgs = argsJSON
	}
	if params.MaxConcurrentTasks != nil {
		dbParams.MaxConcurrentTasks = pgtype.Int4{Int32: *params.MaxConcurrentTasks, Valid: true}
	}
	if params.Visibility != nil {
		dbParams.Visibility = pgtype.Text{String: *params.Visibility, Valid: true}
	}
	if params.AvatarURL != nil {
		dbParams.AvatarUrl = pgtype.Text{String: *params.AvatarURL, Valid: true}
	}

	// --- Handle mcp_config tri-state ---
	if params.MCPConfigPresent {
		dbParams.SetMcpConfig = true
		if string(params.MCPConfig) == "null" {
			dbParams.McpConfig = nil
		} else {
			if !json.Valid(params.MCPConfig) {
				return db.Agent{}, Validation("mcp_config must be valid JSON")
			}
			dbParams.McpConfig = []byte(params.MCPConfig)
		}
	}

	// --- Handle runtime_mode ---
	if params.RuntimeMode != nil {
		dbParams.RuntimeMode = pgtype.Text{String: *params.RuntimeMode, Valid: true}
	}

	// --- Handle provider_id ---
	if setProviderID {
		dbParams.SetProviderID = true
		dbParams.ProviderID = providerUUID
	}

	// --- Handle deliverable_type_id ---
	if setDeliverableTypeID {
		dbParams.SetDeliverableTypeID = true
		dbParams.DeliverableTypeID = deliverableTypeUUID
	}

	// If switching to local mode, clear provider_id
	if params.RuntimeMode != nil && *params.RuntimeMode == "local" && !setProviderID {
		dbParams.SetProviderID = true
		dbParams.ProviderID = pgtype.UUID{} // NULL
	}

	// If switching to online mode, clear runtime_id
	if params.RuntimeMode != nil && *params.RuntimeMode == "online" && params.RuntimeID == nil {
		dbParams.SetRuntimeID = true
		dbParams.RuntimeID = pgtype.UUID{} // NULL
	}

	agent, err := s.q.UpdateAgent(ctx, dbParams)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Agent{}, NotFound("agent not found")
		}
		slog.Error("update agent: query failed", "agent_id", params.AgentID, "user_id", params.UserID, "error", err)
		return db.Agent{}, Internal("failed to update agent")
	}

	slog.Info("agent updated", "agent_id", params.AgentID, "user_id", params.UserID)

	// Broadcast agent_updated event via WebSocket.
	if s.hub != nil {
		s.hub.BroadcastAgentUpdated(agent)
	}

	return agent, nil
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// Delete removes the agent record (scoped to the authenticated user) and
// broadcasts an agent_deleted event. Returns NotFound if the agent does not
// exist or does not belong to the user.
func (s *AgentService) Delete(ctx context.Context, agentID, userID string) *ServiceError {
	agentUUID, err := parseUUID(agentID)
	if err != nil {
		return Validation("invalid agent id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return Internal("invalid user id")
	}

	err = s.q.DeleteAgent(ctx, db.DeleteAgentParams{
		ID:     agentUUID,
		UserID: userUUID,
	})
	if err != nil {
		slog.Error("delete agent: query failed", "agent_id", agentID, "user_id", userID, "error", err)
		return Internal("failed to delete agent")
	}

	slog.Info("agent deleted", "agent_id", agentID, "user_id", userID)

	// Broadcast agent_deleted event via WebSocket.
	if s.hub != nil {
		s.hub.BroadcastAgentDeleted(agentID)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Archive
// ---------------------------------------------------------------------------

// Archive soft-deletes the agent by setting archived_at = now(). The caller
// must be the agent's owner or an admin. Already-archived agents are rejected
// with Conflict.
func (s *AgentService) Archive(ctx context.Context, agentID, userID string, isAdmin bool) (db.Agent, *ServiceError) {
	agentUUID, err := parseUUID(agentID)
	if err != nil {
		return db.Agent{}, Validation("invalid agent id format")
	}

	userUUID, err := parseUUID(userID)
	if err != nil {
		return db.Agent{}, Internal("invalid user id")
	}

	// Fetch the agent to validate existence and check permissions.
	agent, err := s.q.GetAgent(ctx, agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Agent{}, NotFound("agent not found")
		}
		slog.Error("archive agent: get agent failed", "agent_id", agentID, "error", err)
		return db.Agent{}, Internal("failed to get agent")
	}

	// Check if agent is already archived.
	if agent.ArchivedAt.Valid {
		return db.Agent{}, Conflict("agent is already archived")
	}

	// Permission check: user must be the agent owner or an admin.
	isOwner := uuidToString(agent.UserID) == userID
	if !isOwner && !isAdmin {
		return db.Agent{}, Forbidden("you do not have permission to archive this agent")
	}

	// Perform the archive.
	archived, err := s.q.ArchiveAgent(ctx, db.ArchiveAgentParams{
		ID:      agentUUID,
		UserID:  userUUID,
		IsAdmin: isAdmin,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Agent{}, NotFound("agent not found")
		}
		slog.Error("archive agent: query failed", "agent_id", agentID, "user_id", userID, "error", err)
		return db.Agent{}, Internal("failed to archive agent")
	}

	slog.Info("agent archived", "agent_id", agentID, "user_id", userID)

	// Broadcast agent_updated event via WebSocket with updated archived_at.
	if s.hub != nil {
		s.hub.BroadcastAgentUpdated(archived)
	}

	return archived, nil
}

// ---------------------------------------------------------------------------
// Restore
// ---------------------------------------------------------------------------

// Restore clears the archived_at timestamp on an archived agent, effectively
// restoring it to the active view. The agent must exist and be currently
// archived.
func (s *AgentService) Restore(ctx context.Context, agentID string) (db.Agent, *ServiceError) {
	agentUUID, err := parseUUID(agentID)
	if err != nil {
		return db.Agent{}, Validation("invalid agent id format")
	}

	// Fetch the agent to validate it exists and is currently archived.
	agent, err := s.q.GetAgent(ctx, agentUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Agent{}, NotFound("agent not found")
		}
		slog.Error("restore agent: get agent failed", "agent_id", agentID, "error", err)
		return db.Agent{}, Internal("failed to get agent")
	}

	// Verify the agent is currently archived.
	if !agent.ArchivedAt.Valid {
		return db.Agent{}, Conflict("agent is not archived")
	}

	// Restore the agent (set archived_at = NULL).
	restored, err := s.q.RestoreAgent(ctx, agentUUID)
	if err != nil {
		slog.Error("restore agent: query failed", "agent_id", agentID, "error", err)
		return db.Agent{}, Internal("failed to restore agent")
	}

	slog.Info("agent restored", "agent_id", agentID)

	// Broadcast agent_updated event via WebSocket.
	if s.hub != nil {
		s.hub.BroadcastAgentUpdated(restored)
	}

	return restored, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ptrToString safely dereferences a string pointer, returning "" if nil.
func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
