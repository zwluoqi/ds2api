package shared

import (
	"net/http"
	"strings"
)

const ContentFilterFallbackMessage = "【content filter，please update request content】"

func VisibleTextWithContentFilterFallback(text, thinking string, contentFilter bool) string {
	if strings.TrimSpace(text) != "" {
		return text
	}
	if contentFilter || strings.TrimSpace(thinking) != "" {
		return ContentFilterFallbackMessage
	}
	return text
}

func ShouldWriteUpstreamEmptyOutputError(text, thinking string, contentFilter bool) bool {
	_ = contentFilter
	if text != "" {
		return false
	}
	return strings.TrimSpace(thinking) == ""
}

func UpstreamEmptyOutputDetail(contentFilter bool, text, thinking string) (int, string, string) {
	_, _ = text, thinking
	if contentFilter {
		return http.StatusBadRequest, "Upstream content filtered the response and returned no output.", "content_filter"
	}
	return http.StatusTooManyRequests, "Upstream account hit a rate limit and returned empty output.", "upstream_empty_output"
}

func WriteUpstreamEmptyOutputError(w http.ResponseWriter, text, thinking string, contentFilter bool) bool {
	if !ShouldWriteUpstreamEmptyOutputError(text, thinking, contentFilter) {
		return false
	}
	status, message, code := UpstreamEmptyOutputDetail(contentFilter, text, thinking)
	WriteOpenAIErrorWithCode(w, status, message, code)
	return true
}
