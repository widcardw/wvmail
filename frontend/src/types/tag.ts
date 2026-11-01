export type TagMatchField = 'subject' | 'from' | 'domain' | 'body'

export interface MailTag {
  id: string
  label: string
  color: string
  keywords: string[]
  fields: TagMatchField[]
  enabled: boolean
  created_at: number
  updated_at: number
}

export interface EmailTagState {
  selected_tag_ids: string[]
  automatic_tag_ids: string[]
}

export interface TagSuggestionRequest {
  account_ids: string[]
  sample_limit: number
  fields: string[]
  llm_config_name: string
  llm_model: string
}

export interface TagSuggestionPreview {
  prompt: string
  account_names: string[]
  fields: string[]
  sample_count: number
  sample_from: string
  sample_to: string
  llm_name: string
  llm_model: string
}

export interface TagSuggestion {
  label: string
  color: string
  keywords: string[]
  fields: TagMatchField[]
}

export const TAG_FIELDS: TagMatchField[] = ['subject', 'from', 'domain', 'body']

export const AI_TAG_FIELDS = [
  'subject',
  'from_name',
  'from_address',
  'domain',
  'date',
  'body_excerpt',
] as const
