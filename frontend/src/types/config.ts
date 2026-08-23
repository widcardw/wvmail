// TypeScript types matching Go services.AppConfig and services.LLMConfig
// The Go field names are PascalCase and map directly to JS property names.

// export interface AppConfig {
//   Language: string // "en-US" | "zh-CN"
//   Theme: string // "System" | "Light" | "Dark"
//   LLMConfigs: LLMConfig[]
// }
export { AppConfig, LLMConfig } from '#/wvmail/services'

// export interface LLMConfig {
//   Name: string
//   Provider: LLMProvider
//   Model: string
//   BaseURL: string
//   APIKey: string
// }

export type LLMProvider =
  | 'deepseek'
  | 'openai'
  | 'anthropic'
  | 'gemini'
  | 'minimax'
  | 'kimi'
  | 'ollama'
  | 'volcengine'
  | 'custom'

export const LLM_PROVIDER_DEFAULTS: Record<LLMProvider, string> = {
  deepseek: 'https://api.deepseek.com',
  openai: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com',
  gemini: 'https://generativelanguage.googleapis.com',
  minimax: 'https://api.minimax.chat',
  kimi: 'https://api.moonshot.cn',
  ollama: 'http://localhost:11434',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3',
  custom: '',
}

export const LLM_PROVIDER_LABELS: Record<LLMProvider, string> = {
  deepseek: 'DeepSeek',
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  minimax: 'MiniMax',
  kimi: 'Kimi',
  ollama: 'Ollama',
  volcengine: 'Volcengine',
  custom: 'Custom',
}
