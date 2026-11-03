export type AIAssistMode = 'translate' | 'summarize'

/** One provider/model choice made in the privacy confirmation dialog. */
export interface AIAssistSelection {
  engine?: 'llm' | 'deepl'
  llmConfigName?: string
  llmModel?: string
}
