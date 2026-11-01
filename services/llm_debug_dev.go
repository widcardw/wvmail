//go:build !production

package services

import "log/slog"

// Development builds log the complete prompt and raw provider response so JSON
// and reasoning-model failures can be diagnosed. The production build replaces
// these functions with no-ops.
func logLLMRequest(provider, model, prompt string) {
	forceLog(
		"llm: full request",
		slog.String("provider", provider),
		slog.String("model", model),
		slog.String("prompt", prompt),
	)
}

func logLLMResponse(provider, response string) {
	forceLog(
		"llm: full response",
		slog.String("provider", provider),
		slog.String("response", response),
	)
}
