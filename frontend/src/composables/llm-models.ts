import type { LLMConfig } from '~/types/config'

/**
 * Models one config can use: its picked set, or the single legacy `Model`.
 *
 * Configs saved before the catalog feature only carry `Model`, which is shown as
 * the sole entry. The LLM editor, LLM list, and translation settings all agree
 * through this one helper.
 */
export function llmModels(config: LLMConfig | undefined): string[] {
  const models = config?.Models ?? []
  if (models.length > 0) return models
  return config?.Model ? [config.Model] : []
}
