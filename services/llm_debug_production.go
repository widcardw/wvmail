//go:build production

package services

func logLLMRequest(provider, model, prompt string) {}

func logLLMResponse(provider, response string) {}
