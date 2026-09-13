import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'

export type TagDeletePresses = 1 | 2

export const usePreferencesStore = defineStore('preferences', () => {
  const tagDeletePresses = ref<TagDeletePresses>(2)

  function loadFromConfig(config: AppConfig) {
    tagDeletePresses.value = config.SearchTagDeletePresses === 1 ? 1 : 2
  }

  async function setTagDeletePresses(value: TagDeletePresses) {
    tagDeletePresses.value = value
    try {
      await ConfigService.SaveConfigField('search_tag_delete_presses', String(value))
    } catch (error) {
      console.error('Failed to save tag delete preference', error)
    }
  }

  return { tagDeletePresses, loadFromConfig, setTagDeletePresses }
})
