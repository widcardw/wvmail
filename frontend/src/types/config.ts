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

export const DEFAULT_LLM_CONTEXT_TOKENS = 32_000
export const DEFAULT_LLM_OUTPUT_TOKENS = 4096

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

export const LLM_PROVIDER_LIMITS: Record<LLMProvider, { context: number; output: number }> = {
  deepseek: { context: 64_000, output: DEFAULT_LLM_OUTPUT_TOKENS },
  openai: { context: 128_000, output: DEFAULT_LLM_OUTPUT_TOKENS },
  anthropic: { context: 200_000, output: DEFAULT_LLM_OUTPUT_TOKENS },
  gemini: { context: 1_000_000, output: 8192 },
  minimax: { context: 128_000, output: DEFAULT_LLM_OUTPUT_TOKENS },
  kimi: { context: 128_000, output: DEFAULT_LLM_OUTPUT_TOKENS },
  ollama: { context: 8192, output: DEFAULT_LLM_OUTPUT_TOKENS },
  volcengine: { context: DEFAULT_LLM_CONTEXT_TOKENS, output: DEFAULT_LLM_OUTPUT_TOKENS },
  custom: { context: DEFAULT_LLM_CONTEXT_TOKENS, output: DEFAULT_LLM_OUTPUT_TOKENS },
}

export function getLLMProviderLimits(provider?: string) {
  return LLM_PROVIDER_LIMITS[provider as LLMProvider] ?? LLM_PROVIDER_LIMITS.custom
}
