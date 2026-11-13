// Package acp implements the Agent Client Protocol (ACP) for Eino CLI using
// the official Go SDK (github.com/coder/acp-go-sdk).
//
// ACP is an open standard (https://agentclientprotocol.com) that standardizes
// communication between code editors/IDEs (clients) and coding agents.
// Messages are JSON-RPC 2.0 objects, one per line, exchanged over stdio.
package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
	"github.com/coder/acp-go-sdk"
	"github.com/tk103331/eino-cli/agent"
	"github.com/tk103331/eino-cli/config"
	"github.com/tk103331/eino-cli/logger"
	"github.com/tk103331/eino-cli/version"
)

// Service is the Eino CLI ACP agent. It bridges ACP sessions to the
// configured Eino ReAct agent and streams turns to the client.
type Service struct {
	agentName string
	cfg       *config.Config
	conn      *acp.AgentSideConnection

	mu       sync.Mutex
	sessions map[string]*session
	nextSeq  int
}

var _ acp.Agent = (*Service)(nil)

// session holds one ACP conversation context.
type session struct {
	id      string
	cwd     string
	agent   agent.Agent
	history []*schema.Message // prior turns, replayed into every prompt

	running        bool
	toolSeq        int
	lastStartKey   string // consecutive duplicate start events dedupe key
	lastToolCallID acp.ToolCallId
	finalized      bool // whether the current tool call already got a final update
}

// NewService creates an ACP agent bound to the given configured agent name.
func NewService(agentName string, cfg *config.Config) *Service {
	return &Service{
		agentName: agentName,
		cfg:       cfg,
		sessions:  make(map[string]*session),
	}
}

// SetAgentConnection implements acp.AgentConnAware; the SDK calls it right
// after constructing the connection.
func (s *Service) SetAgentConnection(conn *acp.AgentSideConnection) {
	s.conn = conn
}

// Initialize negotiates the protocol version and capabilities.
func (s *Service) Initialize(ctx context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	logger.Info("ACP", fmt.Sprintf("initialize: client=%s requested version=%d",
		clientName(params.ClientInfo), params.ProtocolVersion))

	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersionNumber,
		AgentInfo: &acp.Implementation{
			Name:    "eino-cli",
			Version: version.Version,
		},
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession:        false,
			PromptCapabilities: acp.PromptCapabilities{},
			McpCapabilities:    acp.McpCapabilities{},
		},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

// NewSession creates a session backed by the configured Eino agent.
func (s *Service) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	einoAgent, err := agent.NewFactory(s.cfg).CreateAgent(s.agentName)
	if err != nil {
		logger.Error("ACP", fmt.Sprintf("failed to create agent: %v", err))
		return acp.NewSessionResponse{}, fmt.Errorf("failed to create agent: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSeq++
	sid := fmt.Sprintf("sess-%d", s.nextSeq)
	s.sessions[sid] = &session{
		id:    sid,
		cwd:   params.Cwd,
		agent: einoAgent,
	}
	if len(params.McpServers) > 0 {
		// MCP servers come from config.yml; client-provided ones are ignored.
		names := make([]string, 0, len(params.McpServers))
		for _, server := range params.McpServers {
			names = append(names, mcpServerName(server))
		}
		logger.Warn("ACP", fmt.Sprintf("client-provided MCP servers ignored (using config.yml): %s",
			strings.Join(names, ", ")))
	}

	logger.Info("ACP", fmt.Sprintf("session created: %s (cwd=%s, agent=%s)", sid, params.Cwd, s.agentName))
	return acp.NewSessionResponse{SessionId: acp.SessionId(sid)}, nil
}

// Prompt runs one prompt turn, streaming updates back to the client.
// The SDK cancels the returned context when it receives session/cancel.
func (s *Service) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	sid := string(params.SessionId)

	s.mu.Lock()
	session, ok := s.sessions[sid]
	if ok && session.running {
		s.mu.Unlock()
		return acp.PromptResponse{}, fmt.Errorf("prompt already in progress for session %s", sid)
	}
	if !ok {
		s.mu.Unlock()
		return acp.PromptResponse{}, fmt.Errorf("unknown session: %s", sid)
	}
	session.running = true
	session.toolSeq = 0
	session.lastStartKey = ""
	session.lastToolCallID = ""
	session.finalized = false
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		session.running = false
		s.mu.Unlock()
	}()

	prompt := flattenContentBlocks(params.Prompt)
	if strings.TrimSpace(prompt) == "" {
		return acp.PromptResponse{}, fmt.Errorf("prompt contains no text content")
	}

	logger.Info("ACP", fmt.Sprintf("prompt received: session=%s text=%q", sid, truncate(prompt, 200)))

	// Replay this session's prior turns so the conversation keeps its context.
	s.mu.Lock()
	history := append([]*schema.Message(nil), session.history...)
	s.mu.Unlock()

	var answer strings.Builder
	chunkCallback := s.makeChunkCallback(ctx, sid)
	err := session.agent.ChatStream(ctx, history, prompt,
		func(chunk *agent.StreamChunk) {
			if chunk.Type == "content" && chunk.Content != "" {
				answer.WriteString(chunk.Content)
			}
			chunkCallback(chunk)
		},
		s.makeToolCallback(ctx, sid, session),
	)

	if err != nil {
		if ctx.Err() != nil {
			logger.Info("ACP", fmt.Sprintf("prompt cancelled: session=%s", sid))
			return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		logger.Error("ACP", fmt.Sprintf("prompt failed: session=%s error=%v", sid, err))
		s.sendUpdate(ctx, sid, acp.UpdateAgentMessageText(fmt.Sprintf("\n\n❌ Agent error: %v", err)))
		return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
	}

	// Record the completed turn for the next prompt.
	s.mu.Lock()
	session.history = append(session.history, schema.UserMessage(prompt))
	if text := strings.TrimSpace(answer.String()); text != "" {
		session.history = append(session.history, schema.AssistantMessage(text, nil))
	}
	s.mu.Unlock()

	logger.Info("ACP", fmt.Sprintf("prompt completed: session=%s", sid))
	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

// Cancel aborts the running prompt turn. The SDK has already cancelled the
// prompt context before calling this method.
func (s *Service) Cancel(ctx context.Context, params acp.CancelNotification) error {
	logger.Info("ACP", fmt.Sprintf("cancel received: session=%s", string(params.SessionId)))
	return nil
}

// Authenticate is unused: the agent advertises no auth methods.
func (s *Service) Authenticate(ctx context.Context, params acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// ---- unsupported optional methods ----

func (s *Service) Logout(ctx context.Context, params acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, acp.NewMethodNotFound(acp.AgentMethodLogout)
}

func (s *Service) CloseSession(ctx context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionClose)
}

func (s *Service) ListSessions(ctx context.Context, params acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionList)
}

func (s *Service) ResumeSession(ctx context.Context, params acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionResume)
}

func (s *Service) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
}

func (s *Service) SetSessionMode(ctx context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetMode)
}

// ---- callbacks ----

// makeChunkCallback converts stream chunks into session updates.
// Reasoning is reported as agent_thought_chunk; error chunks are logged but
// not forwarded (the prompt handler emits the final error once).
func (s *Service) makeChunkCallback(ctx context.Context, sid string) func(*agent.StreamChunk) {
	return func(chunk *agent.StreamChunk) {
		switch chunk.Type {
		case "content":
			if chunk.Content == "" {
				return
			}
			s.sendUpdate(ctx, sid, acp.UpdateAgentMessageText(chunk.Content))
		case "reasoning":
			if chunk.Content == "" {
				return
			}
			s.sendUpdate(ctx, sid, acp.UpdateAgentThoughtText(chunk.Content))
		case "error":
			logger.Warn("ACP", fmt.Sprintf("stream error chunk: session=%s %s", sid, chunk.Content))
		}
	}
}

// makeToolCallback converts tool call callbacks into tool_call updates.
// Internal ReAct node events (ChatModel, Tools orchestrator) are filtered out
// so only real tool invocations are reported to the client.
func (s *Service) makeToolCallback(ctx context.Context, sid string, session *session) func(interface{}) {
	return func(data interface{}) {
		info, ok := data.(agent.ToolCallInfo)
		if !ok {
			logger.Debug("ACP", fmt.Sprintf("ignoring tool callback payload: %T", data))
			return
		}
		if isInternalComponent(info.Name) {
			logger.Debug("ACP", fmt.Sprintf("filtered internal component: %s", info.Name))
			return
		}

		switch info.Type {
		case "start":
			s.mu.Lock()
			// The eino callback layer can emit the same tool start event
			// more than once; skip consecutive duplicates.
			startKey := info.Name + "|" + info.Arguments
			if startKey == session.lastStartKey {
				s.mu.Unlock()
				return
			}
			session.lastStartKey = startKey
			session.finalized = false
			session.toolSeq++
			toolCallID := acp.ToolCallId(fmt.Sprintf("call-%s-%d", sid, session.toolSeq))
			session.lastToolCallID = toolCallID
			s.mu.Unlock()

			opts := []acp.ToolCallStartOpt{
				acp.WithStartKind(toolKind(info.Name)),
				acp.WithStartStatus(acp.ToolCallStatusInProgress),
			}
			if info.Arguments != "" && jsonValid(info.Arguments) {
				opts = append(opts, acp.WithStartRawInput(jsonMap(info.Arguments)))
			}
			logger.Info("ACP", fmt.Sprintf("tool_call start: session=%s id=%s name=%s", sid, toolCallID, info.Name))
			s.sendUpdate(ctx, sid, acp.StartToolCall(toolCallID, buildToolTitle(info.Name, info.Arguments), opts...))

		case "end", "error":
			s.mu.Lock()
			toolCallID := session.lastToolCallID
			if toolCallID == "" || session.finalized {
				// No start event seen, or this call already got its final
				// update: the eino callback layer may emit duplicate
				// completion/error events for the same call.
				s.mu.Unlock()
				return
			}
			session.finalized = true
			s.mu.Unlock()

			var update acp.SessionUpdate
			if info.Type == "end" {
				opts := []acp.ToolCallUpdateOpt{
					acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
					acp.WithUpdateContent([]acp.ToolCallContent{
						acp.ToolContent(acp.TextBlock(truncate(info.Result, 8000))),
					}),
				}
				if info.Result != "" && jsonValid(info.Result) {
					opts = append(opts, acp.WithUpdateRawOutput(jsonMap(info.Result)))
				}
				logger.Info("ACP", fmt.Sprintf("tool_call end: session=%s name=%s", sid, info.Name))
				update = acp.UpdateToolCall(toolCallID, opts...)
			} else {
				logger.Warn("ACP", fmt.Sprintf("tool_call error: session=%s name=%s error=%s", sid, info.Name, info.Error))
				update = acp.UpdateToolCall(toolCallID,
					acp.WithUpdateStatus(acp.ToolCallStatusFailed),
					acp.WithUpdateContent([]acp.ToolCallContent{
						acp.ToolContent(acp.TextBlock(truncate("error: "+info.Error, 4000))),
					}),
				)
			}
			s.sendUpdate(ctx, sid, update)
		}
	}
}

// sendUpdate wraps and sends a session/update notification.
func (s *Service) sendUpdate(ctx context.Context, sid string, update acp.SessionUpdate) {
	if err := s.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: acp.SessionId(sid),
		Update:    update,
	}); err != nil {
		logger.Warn("ACP", fmt.Sprintf("failed to send session update: %v", err))
	}
}

// ---- helpers ----

// flattenContentBlocks converts prompt content blocks to plain text for the model.
func flattenContentBlocks(blocks []acp.ContentBlock) string {
	var parts []string
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			if block.Text.Text != "" {
				parts = append(parts, block.Text.Text)
			}
		case block.ResourceLink != nil:
			link := block.ResourceLink.Uri
			if block.ResourceLink.Name != "" {
				link = block.ResourceLink.Name + " (" + block.ResourceLink.Uri + ")"
			}
			parts = append(parts, "[Referenced: "+link+"]")
			if block.ResourceLink.Description != nil && *block.ResourceLink.Description != "" {
				parts = append(parts, *block.ResourceLink.Description)
			}
		case block.Resource != nil:
			res := block.Resource.Resource
			if res.TextResourceContents != nil && res.TextResourceContents.Text != "" {
				parts = append(parts, "[Resource "+res.TextResourceContents.Uri+"]\n"+res.TextResourceContents.Text)
			} else if res.BlobResourceContents != nil {
				mime := ""
				if res.BlobResourceContents.MimeType != nil {
					mime = ", " + *res.BlobResourceContents.MimeType
				}
				parts = append(parts, fmt.Sprintf("[Resource %s%s, %d bytes base64]",
					res.BlobResourceContents.Uri, mime, len(res.BlobResourceContents.Blob)))
			}
		case block.Image != nil, block.Audio != nil:
			parts = append(parts, "[Unsupported content omitted]")
		}
	}
	return strings.Join(parts, "\n")
}

// buildToolTitle renders a short human-readable title for a tool call.
func buildToolTitle(name, args string) string {
	if args == "" || args == "{}" {
		return name
	}
	if m := jsonMap(args); m != nil {
		var kv []string
		for k, v := range m {
			kv = append(kv, fmt.Sprintf("%s=%v", k, v))
		}
		if len(kv) > 0 {
			return fmt.Sprintf("%s %s", name, strings.Join(kv, " "))
		}
	}
	return fmt.Sprintf("%s %s", name, truncate(args, 80))
}

// toolKind maps tool names to ACP tool kinds for better client icons.
func toolKind(name string) acp.ToolKind {
	n := strings.ToLower(name)
	switch {
	case containsAny(n, "commandline", "exec", "shell", "command"):
		return acp.ToolKindExecute
	case containsAny(n, "search", "wikipedia", "duckduckgo", "bing", "google"):
		return acp.ToolKindSearch
	case containsAny(n, "http", "browser", "fetch", "request"):
		return acp.ToolKindFetch
	case containsAny(n, "think", "reason", "plan"):
		return acp.ToolKindThink
	case containsAny(n, "read", "file"):
		return acp.ToolKindRead
	case containsAny(n, "edit", "write"):
		return acp.ToolKindEdit
	default:
		return acp.ToolKindOther
	}
}

// containsAny reports whether s contains any of the substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// truncate cuts a string to maxLen with an ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// isInternalComponent reports whether a callback name refers to an internal
// ReAct node that should not surface as a tool call.
func isInternalComponent(name string) bool {
	return name == "ChatModel" || name == "Tools"
}

// jsonValid reports whether s is valid JSON.
func jsonValid(s string) bool {
	return json.Valid([]byte(s))
}

// jsonMap decodes a JSON object into map[string]any, or nil on failure.
func jsonMap(s string) map[string]any {
	if !json.Valid([]byte(s)) {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// clientName renders the client implementation name for logging.
func clientName(info *acp.Implementation) string {
	if info == nil || info.Name == "" {
		return "unknown"
	}
	if info.Version != "" {
		return info.Name + " " + info.Version
	}
	return info.Name
}

// mcpServerName extracts the display name of a client-provided MCP server.
func mcpServerName(server acp.McpServer) string {
	switch {
	case server.Http != nil:
		return server.Http.Name
	case server.Sse != nil:
		return server.Sse.Name
	case server.Acp != nil:
		return server.Acp.Name
	default:
		return "unnamed"
	}
}
