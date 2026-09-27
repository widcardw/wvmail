import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'

export type TagDeletePresses = 1 | 2

/** Lower-cased comparison key for a sender address. */
function normalizeSender(sender: string): string {
  return sender.trim().toLowerCase()
}

export const usePreferencesStore = defineStore('preferences', () => {
  const tagDeletePresses = ref<TagDeletePresses>(2)
  /** Load remote images/fonts/CSS in every message. Default false — block. */
  const allowRemoteContent = ref(false)
  /** Senders whose remote content always loads (address, or "@domain"). */
  const allowedRemoteSenders = ref<string[]>([])

  const minimizeWhenClose = ref(false)

  function loadFromConfig(config: AppConfig) {
    tagDeletePresses.value = config.SearchTagDeletePresses === 1 ? 1 : 2
    allowRemoteContent.value = config.RemoteContent?.AllowRemoteContent === true
    allowedRemoteSenders.value = [...(config.RemoteContent?.AllowedSenders ?? [])]
    minimizeWhenClose.value = config.MinimizeWhenClose
  }

  async function setTagDeletePresses(value: TagDeletePresses) {
    tagDeletePresses.value = value
    try {
      await ConfigService.SaveConfigField('search_tag_delete_presses', String(value))
    } catch (error) {
      console.error('Failed to save tag delete preference', error)
    }
  }

  async function setMinimizeWhenClose(value: boolean) {
    minimizeWhenClose.value = value
    try {
      await ConfigService.SaveConfigField('minimize_when_close', String(value))
    } catch (error) {
      console.error('Failed to save action on close preference', error)
    }
  }

  async function setAllowRemoteContent(value: boolean) {
    allowRemoteContent.value = value
    try {
      await ConfigService.SaveConfigField('allow_remote_content', String(value))
    } catch (error) {
      console.error('Failed to save remote content preference', error)
    }
  }

  /** True when this sender's remote content may load without asking again. */
  function isRemoteSenderAllowed(sender: string): boolean {
    const key = normalizeSender(sender)
    if (!key) return false
    return allowedRemoteSenders.value.some((entry) =>
      entry.startsWith('@') ? key.endsWith(entry) : key === entry,
    )
  }

  async function allowRemoteSender(sender: string) {
    const key = normalizeSender(sender)
    if (!key || isRemoteSenderAllowed(key)) return
    allowedRemoteSenders.value = [...allowedRemoteSenders.value, key]
    await persistAllowedSenders()
  }

  async function removeAllowedRemoteSender(sender: string) {
    allowedRemoteSenders.value = allowedRemoteSenders.value.filter((entry) => entry !== sender)
    await persistAllowedSenders()
  }

  async function persistAllowedSenders() {
    try {
      await ConfigService.SaveConfigField(
        'allowed_remote_senders',
        JSON.stringify(allowedRemoteSenders.value),
      )
    } catch (error) {
      console.error('Failed to save allowed remote senders', error)
    }
  }

  return {
    tagDeletePresses,
    allowRemoteContent,
    allowedRemoteSenders,
    minimizeWhenClose,
    setMinimizeWhenClose,
    loadFromConfig,
    setTagDeletePresses,
    setAllowRemoteContent,
    isRemoteSenderAllowed,
    allowRemoteSender,
    removeAllowedRemoteSender,
  }
})
