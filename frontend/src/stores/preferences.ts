import { defineStore } from 'pinia'
import { ref, type Ref } from 'vue'
import type { AppConfig } from '~/types/config'
import { ConfigService } from '#/wvmail/services'
import {
  appendRemoteSenderPattern,
  isValidRemoteSenderPattern,
  normalizeSender,
  replaceRemoteSenderPattern,
  senderMatchesPattern,
} from '~/composables/remote-senders'
import type { RemoteSenderListResult } from '~/composables/remote-senders'

export type TagDeletePresses = 1 | 2

export const usePreferencesStore = defineStore('preferences', () => {
  const tagDeletePresses = ref<TagDeletePresses>(2)
  /** Load remote images/fonts/CSS in every message. Default false — block. */
  const allowRemoteContent = ref(false)
  /** Senders whose remote content always loads (address, "@domain", or pattern). */
  const allowedRemoteSenders = ref<string[]>([])
  /**
   * Base URL of the image proxy. Empty uses this app's `/image-proxy`; in the
   * desktop build that is local (it strips cookies/referrer but not the IP), so
   * a remote server-mode deployment should be entered here.
   */
  const imageProxyURL = ref('')

  const minimizeWhenClose = ref(false)
  /** Highlight date/time mentions in email bodies (and offer to remind on click). */
  const detectDatesInMail = ref(true)
  /** Send mail content to the selected AI provider without asking first. */
  const skipAIAssistConfirmation = ref(false)
  /** Render the top smart-folder links as one row of icon buttons. */
  const collapseSidebarSmartFolders = ref(false)
  /** Render the bottom utility links as one row of icon buttons. */
  const collapseSidebarUtilities = ref(false)
  /**
   * Hold suspicious mail behind the "show the body" bar. Default true: the gate
   * has always been on, and a config written before this option existed must keep
   * that behaviour (see `defaultConfig` in services/configservice.go).
   */
  const blockSuspiciousMail = ref(true)

  /**
   * The app-wide proxy (Settings → Proxy). Off by default: a config written
   * before this option existed must keep connecting directly.
   *
   * Held here rather than in a store of its own because every other preference
   * already loads from the same AppConfig and saves through the same field-level
   * binding — a second store would duplicate both halves of that.
   */
  const proxyEnabled = ref(false)
  const proxyUrl = ref('')

  function loadFromConfig(config: AppConfig) {
    tagDeletePresses.value = config.SearchTagDeletePresses === 1 ? 1 : 2
    allowRemoteContent.value = config.RemoteContent?.AllowRemoteContent === true
    allowedRemoteSenders.value = [...(config.RemoteContent?.AllowedSenders ?? [])]
    imageProxyURL.value = config.RemoteContent?.ImageProxyURL ?? ''
    minimizeWhenClose.value = config.MinimizeWhenClose
    detectDatesInMail.value = config.DetectDatesInMail !== false
    skipAIAssistConfirmation.value = config.SkipAIAssistConfirmation === true
    collapseSidebarSmartFolders.value = config.CollapseSidebarSmartFolders === true
    collapseSidebarUtilities.value = config.CollapseSidebarUtilities === true
    blockSuspiciousMail.value = config.ScanExternalSafety !== false
    proxyEnabled.value = config.Proxy?.Enabled === true
    proxyUrl.value = config.Proxy?.URL ?? ''
  }

  async function saveConfigField(field: string, value: string, failureMessage: string) {
    try {
      await ConfigService.SaveConfigField(field, value)
    } catch (error) {
      console.error(failureMessage, error)
    }
  }

  /**
   * Most preferences are optimistic local state plus one scalar config field.
   * Keep that write path in one place so each setter cannot drift on string
   * conversion or error reporting.
   */
  function persistedSetting<T extends string | number | boolean>(
    target: Ref<T>,
    field: string,
    failureMessage: string,
  ) {
    return async (value: T) => {
      target.value = value
      await saveConfigField(field, String(value), failureMessage)
    }
  }

  const setTagDeletePresses = persistedSetting(
    tagDeletePresses,
    'search_tag_delete_presses',
    'Failed to save tag delete preference',
  )
  const setMinimizeWhenClose = persistedSetting(
    minimizeWhenClose,
    'minimize_when_close',
    'Failed to save action on close preference',
  )
  const setDetectDatesInMail = persistedSetting(
    detectDatesInMail,
    'detect_dates_in_mail',
    'Failed to save date detection preference',
  )
  const setSkipAIAssistConfirmation = persistedSetting(
    skipAIAssistConfirmation,
    'skip_ai_assist_confirmation',
    'Failed to save AI assist confirmation preference',
  )
  const setCollapseSidebarSmartFolders = persistedSetting(
    collapseSidebarSmartFolders,
    'collapse_sidebar_smart_folders',
    'Failed to save smart-folder sidebar preference',
  )
  const setCollapseSidebarUtilities = persistedSetting(
    collapseSidebarUtilities,
    'collapse_sidebar_utilities',
    'Failed to save utility sidebar preference',
  )
  const setAllowRemoteContent = persistedSetting(
    allowRemoteContent,
    'allow_remote_content',
    'Failed to save remote content preference',
  )
  const setBlockSuspiciousMail = persistedSetting(
    blockSuspiciousMail,
    // The YAML key predates this switch's move to Settings → Security; renaming
    // it would silently drop the choice from every existing config.
    'scan_external_safety',
    'Failed to save suspicious mail preference',
  )

  /**
   * Save the global proxy. Both halves travel in one field: they are one
   * decision, and two separate writes could leave "enabled with no address" on
   * disk after a crash between them.
   *
   * Unlike the other setters this one rethrows — the address is validated on the
   * Go side, and the settings page has to be able to say so instead of leaving a
   * switch that looks on but was never written.
   */
  async function setProxy(enabled: boolean, url: string) {
    const previous = { enabled: proxyEnabled.value, url: proxyUrl.value }
    proxyEnabled.value = enabled
    proxyUrl.value = url
    try {
      await ConfigService.SaveConfigField('proxy', JSON.stringify({ enabled, url }))
    } catch (error) {
      proxyEnabled.value = previous.enabled
      proxyUrl.value = previous.url
      throw error
    }
  }

  /** True when this sender's remote content may load without asking again. */
  function isRemoteSenderAllowed(sender: string): boolean {
    const key = normalizeSender(sender)
    if (!key) return false
    return allowedRemoteSenders.value.some((entry) => senderMatchesPattern(key, entry))
  }
  async function addRemoteSenderPattern(sender: string): Promise<RemoteSenderListResult['status']> {
    const result = appendRemoteSenderPattern(allowedRemoteSenders.value, sender)
    if (result.status !== 'added') return result.status
    allowedRemoteSenders.value = result.senders
    await persistAllowedSenders()
    return result.status
  }

  async function updateAllowedRemoteSender(
    oldSender: string,
    newSender: string,
  ): Promise<RemoteSenderListResult['status']> {
    const result = replaceRemoteSenderPattern(allowedRemoteSenders.value, oldSender, newSender)
    if (result.status !== 'updated') return result.status
    allowedRemoteSenders.value = result.senders
    await persistAllowedSenders()
    return result.status
  }

  async function allowRemoteSender(sender: string) {
    const key = normalizeSender(sender)
    if (!key || isRemoteSenderAllowed(key)) return
    // The reader's button always supplies a real address; save it unchanged.
    const result = appendRemoteSenderPattern(allowedRemoteSenders.value, key)
    if (result.status !== 'added') return
    allowedRemoteSenders.value = result.senders
    await persistAllowedSenders()
  }

  async function removeAllowedRemoteSender(sender: string) {
    const key = normalizeSender(sender)
    allowedRemoteSenders.value = allowedRemoteSenders.value.filter(
      (entry) => normalizeSender(entry) !== key,
    )
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

  /**
   * Save the proxy base. Like `setProxy`, this rethrows: the URL is validated on
   * the Go side and the settings field must be able to show the error instead of
   * silently keeping a value that was never written.
   */
  async function setImageProxyURL(value: string) {
    const previous = imageProxyURL.value
    imageProxyURL.value = value
    try {
      await ConfigService.SaveConfigField('image_proxy_url', value)
    } catch (error) {
      imageProxyURL.value = previous
      throw error
    }
  }

  return {
    tagDeletePresses,
    allowRemoteContent,
    allowedRemoteSenders,
    imageProxyURL,
    minimizeWhenClose,
    setMinimizeWhenClose,
    detectDatesInMail,
    setDetectDatesInMail,
    skipAIAssistConfirmation,
    setSkipAIAssistConfirmation,
    collapseSidebarSmartFolders,
    setCollapseSidebarSmartFolders,
    collapseSidebarUtilities,
    setCollapseSidebarUtilities,
    blockSuspiciousMail,
    setBlockSuspiciousMail,
    proxyEnabled,
    proxyUrl,
    setProxy,
    loadFromConfig,
    setTagDeletePresses,
    setAllowRemoteContent,
    setImageProxyURL,
    isRemoteSenderAllowed,
    isValidRemoteSenderPattern,
    addRemoteSenderPattern,
    updateAllowedRemoteSender,
    allowRemoteSender,
    removeAllowedRemoteSender,
  }
})
