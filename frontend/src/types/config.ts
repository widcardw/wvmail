// TypeScript types matching Go services.AppConfig and services.LLMConfig
// The Go field names are PascalCase and map directly to JS property names.

export interface AppConfig {
  Language: string   // "en-US" | "zh-CN"
  Theme: string      // "System" | "Light" | "Dark"
  LLMConfigs: LLMConfig[]
}

export interface LLMConfig {
  Name: string
  Provider: LLMProvider
  BaseURL: string
  APIKey: string
}

export type LLMProvider =
  | 'deepseek'
  | 'openai'
  | 'anthropic'
  | 'gemini'
  | 'minimax'
  | 'kimi'
  | 'custom'

export const LLM_PROVIDER_DEFAULTS: Record<LLMProvider, string> = {
  deepseek: 'https://api.deepseek.com',
  openai: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com',
  gemini: 'https://generativelanguage.googleapis.com',
  minimax: 'https://api.minimax.chat',
  kimi: 'https://api.moonshot.cn',
  custom: '',
}

export const LLM_PROVIDER_LABELS: Record<LLMProvider, string> = {
  deepseek: 'DeepSeek',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  minimax: 'MiniMax',
  kimi: 'Kimi',
  custom: 'Custom',
}
