package responses

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"ds2api/internal/accountstats"
	"ds2api/internal/auth"
	"ds2api/internal/chathistory"
	"ds2api/internal/config"
	"ds2api/internal/httpapi/openai/files"
	"ds2api/internal/httpapi/openai/history"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/promptcompat"
	"ds2api/internal/toolcall"
	"ds2api/internal/toolstream"
)

const openAIGeneralMaxSize = shared.GeneralMaxSize

var writeJSON = shared.WriteJSON

type Handler struct {
	Store       shared.ConfigReader
	Auth        shared.AuthResolver
	DS          shared.DeepSeekCaller
	ChatHistory *chathistory.Store
	Stats       *accountstats.Store

	responsesMu sync.Mutex
	responses   *responseStore
}

func (h *Handler) compatStripReferenceMarkers() bool {
	if h == nil {
		return true
	}
	return shared.CompatStripReferenceMarkers(h.Store)
}

func (h *Handler) applyCurrentInputFile(ctx context.Context, a *auth.RequestAuth, stdReq promptcompat.StandardRequest) (promptcompat.StandardRequest, error) {
	if h == nil {
		return stdReq, nil
	}
	stdReq = shared.ApplyThinkingInjection(h.Store, stdReq)
	svc := history.Service{Store: h.Store, DS: h.DS}
	out, err := svc.ApplyCurrentInputFile(ctx, a, stdReq)
	if err != nil || out.CurrentInputFileApplied {
		return out, err
	}
	return out, nil
}

func (h *Handler) preprocessInlineFileInputs(ctx context.Context, a *auth.RequestAuth, req map[string]any) error {
	if h == nil {
		return nil
	}
	return (&files.Handler{Store: h.Store, Auth: h.Auth, DS: h.DS, ChatHistory: h.ChatHistory}).PreprocessInlineFileInputs(ctx, a, req)
}

type accountFilteringAuthResolver interface {
	DetermineWithAccountFilter(req *http.Request, accept func(config.Account) bool) (*auth.RequestAuth, error)
}

func (h *Handler) determineAuthForModel(r *http.Request, model string) (*auth.RequestAuth, error) {
	if h == nil || h.Auth == nil {
		return nil, auth.ErrUnauthorized
	}
	resolver, ok := h.Auth.(accountFilteringAuthResolver)
	if !ok {
		return h.Auth.Determine(r)
	}
	return resolver.DetermineWithAccountFilter(r, func(acc config.Account) bool {
		return shared.AccountWithinTotalLimits(h.Stats, acc, model)
	})
}

func resolveRequestModelForLimits(store shared.ConfigReader, req map[string]any) (string, bool) {
	model, _ := req["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		return "", true
	}
	return config.ResolveModel(store, model)
}

func (h *Handler) toolcallFeatureMatchEnabled() bool {
	if h == nil {
		return shared.ToolcallFeatureMatchEnabled(nil)
	}
	return shared.ToolcallFeatureMatchEnabled(h.Store)
}

func (h *Handler) toolcallEarlyEmitHighConfidence() bool {
	if h == nil {
		return shared.ToolcallEarlyEmitHighConfidence(nil)
	}
	return shared.ToolcallEarlyEmitHighConfidence(h.Store)
}

func writeOpenAIError(w http.ResponseWriter, status int, message string) {
	shared.WriteOpenAIError(w, status, message)
}

func writeOpenAIErrorWithCode(w http.ResponseWriter, status int, message, code string) {
	shared.WriteOpenAIErrorWithCode(w, status, message, code)
}

func openAIErrorType(status int) string {
	return shared.OpenAIErrorType(status)
}

func writeOpenAIInlineFileError(w http.ResponseWriter, err error) {
	files.WriteInlineFileError(w, err)
}

func mapCurrentInputFileError(err error) (int, string) {
	return history.MapError(err)
}

func requestTraceID(r *http.Request) string {
	return shared.RequestTraceID(r)
}

func cleanVisibleOutput(text string, stripReferenceMarkers bool) string {
	return shared.CleanVisibleOutput(text, stripReferenceMarkers)
}

func replaceCitationMarkersWithLinks(text string, links map[int]string) string {
	return shared.ReplaceCitationMarkersWithLinks(text, links)
}

func visibleTextWithContentFilterFallback(text, thinking string, contentFilter bool) string {
	return shared.VisibleTextWithContentFilterFallback(text, thinking, contentFilter)
}

func upstreamEmptyOutputDetail(contentFilter bool, text, thinking string) (int, string, string) {
	return shared.UpstreamEmptyOutputDetail(contentFilter, text, thinking)
}

func writeUpstreamEmptyOutputError(w http.ResponseWriter, text, thinking string, contentFilter bool) bool {
	return shared.WriteUpstreamEmptyOutputError(w, text, thinking, contentFilter)
}

func (h *Handler) emptyOutputRetryEnabled() bool {
	if h == nil {
		return false
	}
	return shared.EmptyOutputRetryEnabled(h.Store)
}

func (h *Handler) emptyOutputRetryMaxAttempts() int {
	if h == nil {
		return 0
	}
	return shared.EmptyOutputRetryMaxAttempts(h.Store)
}

func clonePayloadForEmptyOutputRetry(payload map[string]any, parentMessageID int) map[string]any {
	return shared.ClonePayloadForEmptyOutputRetry(payload, parentMessageID)
}

func usagePromptWithEmptyOutputRetry(originalPrompt string, retryAttempts int) string {
	return shared.UsagePromptWithEmptyOutputRetry(originalPrompt, retryAttempts)
}

func filterIncrementalToolCallDeltasByAllowed(deltas []toolstream.ToolCallDelta, seenNames map[int]string) []toolstream.ToolCallDelta {
	return shared.FilterIncrementalToolCallDeltasByAllowed(deltas, seenNames)
}

func detectAssistantToolCalls(rawText, visibleText, exposedThinking, detectionThinking string, toolNames []string) toolcall.ToolCallParseResult {
	return shared.DetectAssistantToolCalls(rawText, visibleText, exposedThinking, detectionThinking, toolNames)
}
