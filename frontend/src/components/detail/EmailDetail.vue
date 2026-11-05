<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useThemeStore } from '~/stores/theme'
import { useFeedbackStore } from '~/stores/feedback'
import { usePreferencesStore } from '~/stores/preferences'
import { useLocaleStore } from '~/stores/locale'
import { formatDate } from '~/composables/formats'
import { sanitizeEmailHtml, sanitizePlainTextEmail } from '~/composables/html'
import { isSpamGated } from '~/composables/mail-safety'
import { useMailAIAssist } from '~/composables/mail-ai-assist'
import { buildReplyEnvelope, escapeHTML, type ReplyMode } from '~/composables/reply'
import { ComposeService, EmailService, OsService, TagService } from '#/wvmail/services'
import { useTagsStore } from '~/stores/tags'
import type { AIAssistMode, AIAssistSelection } from '~/types/ai-assist'
import EmailShadowBody from './EmailShadowBody.vue'
import EmailActionBar from './EmailActionBar.vue'
import EmailHeader from './EmailHeader.vue'
import EmailAIAssistBody from './EmailAIAssistBody.vue'
import AIAssistConfirmDialog from './AIAssistConfirmDialog.vue'
import EmailAttachments from './EmailAttachments.vue'
import EmailRemoteNotice from './EmailRemoteNotice.vue'
import EmailCssArtNotice from './EmailCssArtNotice.vue'
import EmailSecurityNotice from './EmailSecurityNotice.vue'
import EmailLinkDialog from './EmailLinkDialog.vue'
import RemindFromMailDialog from './RemindFromMailDialog.vue'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
import UiAlert from '~/components/ui/UiAlert.vue'
import TagChip from '~/components/tags/TagChip.vue'
import EmailTagDialog from '~/components/tags/EmailTagDialog.vue'

const { t, tf } = useI18n()
const accountsStore = useAccountsStore()
const themeStore = useThemeStore()
const fb = useFeedbackStore()
const preferences = usePreferencesStore()
const localeStore = useLocaleStore()
const tagsStore = useTagsStore()

const isDarkMode = computed(() => themeStore.effectiveTheme === 'dark')
const detail = computed(() => accountsStore.selectedEmailDetail)
const loading = computed(() => accountsStore.loadingDetail)
const exportingOriginal = ref(false)

const suspiciousRevealed = ref(false)
const attachmentsOpen = ref(false)
const deleteOpen = ref(false)
const tagDialogOpen = ref(false)
const aiConfirmOpen = ref(false)
const aiConfirmMode = ref<AIAssistMode>('translate')
const selectedEmailTagIDs = ref<string[]>([])
let tagStateRequestId = 0
/** Revealed for this message only; reset whenever another email is opened. */
const remoteRevealed = ref(false)

/**
 * Whether this message stays behind the spam gate.
 *
 * One definition for the six places that used to repeat
 * `detail.is_suspicious && !suspiciousRevealed`: the bar, the body, the
 * attachments and the two sanitiser notices all have to agree, and the Settings
 * switch (Settings → Security) has to reach all of them or none.
 */
const spamGated = computed(() =>
  isSpamGated(
    Boolean(detail.value?.is_suspicious),
    preferences.blockSuspiciousMail,
    suspiciousRevealed.value,
  ),
)

const senderKey = computed(() => (detail.value?.from_addr ?? '').toLowerCase())
const senderRemoteAllowed = computed(() => preferences.isRemoteSenderAllowed(senderKey.value))
// Remote content loads only when the user allowed it globally, allowed this
// sender, or asked for this one message.
const allowRemote = computed(
  () => preferences.allowRemoteContent || senderRemoteAllowed.value || remoteRevealed.value,
)

// Sanitising parses and rewrites the whole body, so cache it per detail (and per
// remote-content decision) instead of re-running it on every re-render.
const sanitized = computed(() => {
  const mail = detail.value
  if (!mail) return { html: '', blockedRemote: 0, cssArt: 0 }
  if (mail.body_is_plain_text || !mail.body_html) {
    if (!mail.body_text) return { html: '', blockedRemote: 0, cssArt: 0 }
    return sanitizePlainTextEmail(mail.body_text, sanitizeOptions())
  }
  return sanitizeEmailHtml(mail.body_html, sanitizeOptions())
})
const sanitizedBodyHtml = computed(() => sanitized.value.html)
const blockedRemote = computed(() => sanitized.value.blockedRemote)
/** Style declarations dropped because they were spam-grade (a picture drawn with CSS). */
const cssArt = computed(() => sanitized.value.cssArt)

/**
 * Reference for date detection: when the message was sent, not the clock. A mail
 * from last month saying "明天" must resolve to the day after *that* day, and its
 * own timestamps must not look like history just because we read it later.
 */
function mailReferenceDate(): Date {
  const raw = detail.value?.date
  const sent = raw ? new Date(raw) : null
  return sent && !Number.isNaN(sent.getTime()) ? sent : new Date()
}

/** Shared sanitize options: blocked-image tooltips follow the UI language. */
function sanitizeOptions() {
  return {
    allowRemote: allowRemote.value,
    blockedImageLabel: t.value['mail.remote.blocked_image'],
    // Remote images are fetched by the proxy, never by the webview. Empty means
    // this app's own endpoint; a server-mode deployment hides the IP, while the
    // desktop build's local endpoint only strips cookies/referrer.
    imageProxyURL: preferences.imageProxyURL || '/image-proxy',
    // The mail's own `prefers-color-scheme` blocks follow our theme, not the OS
    // (reading it here also makes the `sanitized` computed re-run on a theme switch).
    darkMode: isDarkMode.value,
    // Highlight dates/times in the body; the tooltip previews what a click would
    // create, so an over-eager match is visible before it is clicked.
    mentions: {
      enabled: preferences.detectDatesInMail,
      locale: localeStore.current,
      now: mailReferenceDate(),
      label: (at: Date) =>
        `${t.value['mail.remind.hover']}${formatDate(at.toISOString(), localeStore.current)}`,
    },
  }
}

// Translation and summary share one state machine so the privacy dialog, the
// toolbar and the result panels cannot disagree about what is in flight.
const {
  translating,
  translatedText,
  translatedHtml,
  translationError,
  summarizing,
  summaryHtml,
  summaryError,
  hasTranslation,
  hasTranslationOutput,
  hasSummary,
  hasSummaryOutput,
  clearTranslation,
  cancelTranslation,
  startTranslation,
  clearSummary,
  cancelSummary,
  startSummary,
} = useMailAIAssist({
  detail,
  sanitizeHtml: (html) => sanitizeEmailHtml(html, sanitizeOptions()).html,
  sanitizeText: (text) => sanitizePlainTextEmail(text, sanitizeOptions()).html,
  blocked: () => spamGated.value,
})

watch(detail, () => {
  cancelTranslation()
  cancelSummary()
  suspiciousRevealed.value = false
  remoteRevealed.value = false
  attachmentsOpen.value = false
  remindOpen.value = false
  deleteOpen.value = false
  tagDialogOpen.value = false
  aiConfirmOpen.value = false
})

function requestAIAssist(mode: AIAssistMode) {
  if (preferences.skipAIAssistConfirmation) {
    runAIAssist(mode, {})
    return
  }
  aiConfirmMode.value = mode
  aiConfirmOpen.value = true
}

function runAIAssist(mode: AIAssistMode, selection: AIAssistSelection) {
  if (mode === 'translate') {
    void startTranslation(selection)
    return
  }
  void startSummary(selection)
}

function toggleTranslation() {
  if (translating.value) {
    cancelTranslation()
    return
  }
  if (hasTranslation.value) {
    clearTranslation()
    return
  }
  requestAIAssist('translate')
}

function toggleSummary() {
  if (summarizing.value) {
    cancelSummary()
    return
  }
  if (hasSummary.value) {
    clearSummary()
    return
  }
  requestAIAssist('summarize')
}

function confirmAIAssist(selection: AIAssistSelection, dontAsk: boolean) {
  if (dontAsk) void preferences.setSkipAIAssistConfirmation(true)
  aiConfirmOpen.value = false
  runAIAssist(aiConfirmMode.value, selection)
}

watch(
  () => [
    accountsStore.selectedEmailAccountId,
    accountsStore.selectedEmailFolder,
    accountsStore.selectedEmailUid,
  ],
  async () => {
    const requestId = ++tagStateRequestId
    selectedEmailTagIDs.value = []
    const accountId = accountsStore.selectedEmailAccountId
    const folder = accountsStore.selectedEmailFolder
    const uid = accountsStore.selectedEmailUid
    if (!accountId || !folder || uid == null) return
    try {
      if (tagsStore.tags.length === 0) await tagsStore.loadTags()
      const state = await TagService.GetEmailTagState(accountId, folder, uid)
      if (requestId === tagStateRequestId) selectedEmailTagIDs.value = [...state.selected_tag_ids]
    } catch (error) {
      console.error('Failed to load tags for the open message', error)
    }
  },
  { immediate: true },
)

const selectedEmailTags = computed(() =>
  selectedEmailTagIDs.value.map((id) => tagsStore.tagByID[id]).filter(Boolean),
)

function onEmailTagsSaved(tagIds: string[]) {
  selectedEmailTagIDs.value = [...tagIds]
}

/** Load this message's remote content once (nothing is remembered). */
function revealRemoteContent() {
  remoteRevealed.value = true
}

/** Load remote content for every future message from this sender. */
async function alwaysAllowSender() {
  if (!senderKey.value) return
  remoteRevealed.value = true
  await preferences.allowRemoteSender(senderKey.value)
  fb.success(t.value['mail.remote.allowed'])
}

async function handleRefresh() {
  // Drop stale AI output so a re-fetch is always actionable.
  cancelTranslation()
  cancelSummary()
  const account = accountsStore.selectedEmailAccount
  const folder = accountsStore.selectedEmailFolder
  const uid = accountsStore.selectedEmailUid
  if (!account || !folder || uid == null) {
    fb.info(t.value['mail.refresh.aggregate'])
    return
  }
  await accountsStore.refreshEmailBody(account, folder, uid)
}

async function handleExportOriginal() {
  const account = accountsStore.selectedEmailAccount
  const folder = accountsStore.selectedEmailFolder
  const uid = accountsStore.selectedEmailUid
  if (!account || !folder || uid == null) {
    fb.info(t.value['mail.export.aggregate'])
    return
  }

  exportingOriginal.value = true
  try {
    const filename = `${detail.value?.subject || 'email'}.eml`
    const savePath = await OsService.SaveEmailDialog(t.value['mail.export.dialog'], filename)
    await EmailService.ExportOriginalMessage(account, folder, uid, savePath)
    fb.success(t.value['mail.export.saved'])
  } catch (e) {
    if (String(e).toLowerCase().includes('cancel')) return
    fb.error(t.value['mail.export.failed'])
    console.error('Failed to export original email', e)
  } finally {
    exportingOriginal.value = false
  }
}

async function handleDelete() {
  const deleted = await accountsStore.deleteSelectedEmail()
  if (deleted) deleteOpen.value = false
}

function quotedBodyHTML(mail: NonNullable<typeof detail.value>): string {
  if (spamGated.value) return ''
  if (sanitizedBodyHtml.value) return `<blockquote>${sanitizedBodyHtml.value}</blockquote>`
  if (!mail.body_text) return ''
  return `<blockquote><p>${escapeHTML(mail.body_text).replaceAll('\n', '<br>')}</p></blockquote>`
}

function replyBodyHTML(
  mail: NonNullable<typeof detail.value>,
  mode: ReplyMode,
): { html: string; text: string } {
  const sender = mail.from_name || mail.from_addr || mail.from
  const date = formatDate(mail.date, localeStore.current)
  const quote = quotedBodyHTML(mail)
  if (mode !== 'forward') {
    const intro = tf('compose.reply_intro', { date, sender })
    return {
      html: `<p><br></p><p>${escapeHTML(intro)}</p>${quote}`,
      text: `\n\n${intro}\n${mail.body_text}`,
    }
  }

  const recipients = [...(mail.to ?? []), ...(mail.cc ?? [])].join(', ')
  const rows = [
    [t.value['compose.forward_from'], sender],
    [t.value['compose.forward_date'], date],
    [t.value['compose.forward_subject'], mail.subject || t.value['mail.noSubject']],
    [t.value['compose.forward_to'], recipients],
  ]
  const htmlRows = rows
    .map(([label, value]) => `${escapeHTML(label)}: ${escapeHTML(value)}`)
    .join('<br>')
  const textRows = rows.map(([label, value]) => `${label}: ${value}`).join('\n')
  return {
    html: `<p><br></p><p>${escapeHTML(t.value['compose.forward_header'])}</p><p>${htmlRows}</p>${quote}`,
    text: `\n\n${t.value['compose.forward_header']}\n${textRows}\n\n${mail.body_text}`,
  }
}

async function openReply(mode: ReplyMode) {
  const mail = detail.value
  if (!mail) return
  const envelope = buildReplyEnvelope(mail, mode, accountsStore.selectedEmailAccount?.Email ?? '')
  if (mode !== 'forward' && envelope.To.length === 0) {
    fb.error(t.value['mail.reply_unavailable'])
    return
  }
  const body = replyBodyHTML(mail, mode)
  try {
    const result = await ComposeService.OpenCompose({
      AccountID: accountsStore.selectedEmailAccountId ?? accountsStore.selectedAccountId ?? '',
      ...envelope,
      Text: body.text,
      HTML: body.html,
    })
    if (result.Warning) fb.info(result.Warning)
  } catch (e) {
    fb.error(t.value['compose.open_failed'])
    console.error('Failed to open reply compose draft', e)
  }
}

const linkOpen = ref(false)
const pendingUrl = ref('')

// A date/time mention clicked in the body: the epoch comes from the span the
// sanitizer added, everything else is editable in the dialog.
const remindOpen = ref(false)
const remindAt = ref(0)
const remindText = ref('')
/** Timezone the sentence named, when the detector converted from it. */
const remindZone = ref('')

/** First element on the event path that the detector tagged as a moment. */
function findMention(path: EventTarget[]): HTMLElement | null {
  for (const node of path) {
    if (node instanceof HTMLElement && node.dataset.remindAt) return node
  }
  return null
}

async function handleLinkClick(event: MouseEvent) {
  const path: EventTarget[] = (event as any).composedPath?.() ?? []
  const mention = findMention(path)
  if (mention) {
    event.preventDefault()
    const epoch = Number(mention.dataset.remindAt)
    if (!Number.isFinite(epoch) || epoch <= 0) return
    remindAt.value = epoch
    remindText.value = mention.dataset.remindText || mention.textContent || ''
    remindZone.value = mention.dataset.remindZone || ''
    remindOpen.value = true
    return
  }

  const target = (path[0] ?? event.target) as HTMLElement | null
  if (!target) return
  let el: HTMLElement | null = target
  while (el && el.tagName !== 'A') el = el.parentElement
  if (!el || !(el instanceof HTMLAnchorElement)) return
  const href = el.getAttribute('href')
  if (!href) return
  if (href.toLowerCase().startsWith('mailto:')) {
    event.preventDefault()
    try {
      const result = await ComposeService.OpenMailto(
        el.href,
        accountsStore.selectedEmailAccountId ?? accountsStore.selectedAccountId ?? '',
      )
      if (result.Warning) fb.info(result.Warning)
    } catch (e) {
      fb.error(t.value['compose.open_failed'])
      console.error('Failed to open mailto compose draft', e)
    }
    return
  }
  if (href.startsWith('#') && !href.startsWith('http')) return

  event.preventDefault()
  pendingUrl.value = href.startsWith('http') ? href : el.href
  linkOpen.value = true
}

function closeError() {
  accountsStore.detailError = null
}

const showAttachments = computed(
  () => detail.value?.attachments && detail.value.attachments.length > 0 && !spamGated.value,
)
</script>

<template>
  <div class="email-detail">
    <EmailActionBar
      :has-detail="!!detail"
      :loading="loading"
      :exporting-original="exportingOriginal"
      :translating="translating"
      :has-translation="hasTranslation"
      :summarizing="summarizing"
      :has-summary="hasSummary"
      :ai-blocked="spamGated"
      :flagged="accountsStore.selectedEmailFlagged"
      :flagging="accountsStore.selectedEmailFlagging"
      :deleting="accountsStore.deletingSelectedEmail"
      @reply="openReply('reply')"
      @reply-all="openReply('reply-all')"
      @forward="openReply('forward')"
      @refresh="handleRefresh"
      @export-original="handleExportOriginal"
      @toggle-translation="toggleTranslation"
      @toggle-summary="toggleSummary"
      @toggle-flag="accountsStore.toggleSelectedEmailFlagged"
      @edit-tags="tagDialogOpen = true"
      @delete="deleteOpen = true"
    />

    <UiScrollArea class="scroll">
      <!-- fetch / AI assist errors (the spam gate lives in the body-bar stack below) -->
      <UiAlert
        v-if="accountsStore.detailError"
        class="detail-alert"
        tone="error"
        dismissible
        :close-label="t['cancel']"
        @dismiss="closeError()"
      >
        {{ accountsStore.detailError }}
      </UiAlert>

      <div v-if="!detail && !loading" class="centerpad empty">{{ t['mail.selectFolder'] }}</div>

      <template v-if="detail">
        <EmailHeader :detail="detail">
          <template #after-meta>
            <div v-if="selectedEmailTags.length" class="detail-tags">
              <button
                v-for="tag in selectedEmailTags"
                :key="tag.id"
                type="button"
                class="detail-tags__button"
                :title="t['mail.tags.edit']"
                @click="tagDialogOpen = true"
              >
                <TagChip :tag="tag" />
              </button>
            </div>
          </template>
        </EmailHeader>

        <!-- One bar stack above the body: the spam gate, then what the sanitiser dropped. -->
        <EmailSecurityNotice v-if="spamGated" @reveal="suspiciousRevealed = true" />

        <EmailCssArtNotice v-if="cssArt > 0 && !spamGated" :count="cssArt" />

        <EmailRemoteNotice
          v-if="blockedRemote > 0 && !spamGated"
          :count="blockedRemote"
          :can-always-allow="Boolean(senderKey)"
          @reveal="revealRemoteContent()"
          @always="alwaysAllowSender()"
        />

        <EmailAttachments
          v-if="showAttachments"
          :attachments="detail.attachments!"
          :open="attachmentsOpen"
          @update:open="attachmentsOpen = $event"
        />

        <div class="detail-body" @click="handleLinkClick">
          <EmailShadowBody
            v-if="sanitizedBodyHtml && !spamGated"
            :html="sanitizedBodyHtml"
            :dark="isDarkMode"
          />
          <div v-else class="centerpad-empty">{{ t['mail.noContent'] }}</div>
        </div>

        <EmailAIAssistBody
          v-if="hasTranslationOutput && !spamGated"
          :label="t['mail.translation.label']"
          :html="translatedHtml"
          :text="translatedText"
          :error="translationError"
          :dark="isDarkMode"
          @dismiss-error="translationError = null"
        />

        <EmailAIAssistBody
          v-if="hasSummaryOutput && !spamGated"
          :label="t['mail.summary.label']"
          :html="summaryHtml"
          :text="null"
          :error="summaryError"
          :dark="isDarkMode"
          @dismiss-error="summaryError = null"
        />
      </template>
    </UiScrollArea>

    <div v-if="loading" class="detail-progress" aria-hidden="true" />
  </div>
  <Teleport to="body">
    <EmailLinkDialog v-model:open="linkOpen" :url="pendingUrl" />
    <AIAssistConfirmDialog
      v-model:open="aiConfirmOpen"
      :mode="aiConfirmMode"
      @confirm="confirmAIAssist"
    />
    <RemindFromMailDialog
      v-model:open="remindOpen"
      :at="remindAt"
      :text="remindText"
      :zone="remindZone"
      :suggested-title="detail?.subject || ''"
    />
    <ConfirmDialog
      :open="deleteOpen"
      :title="t['mail.delete']"
      :description="t['mail.delete.confirm_body']"
      :confirm-label="t['mail.delete']"
      :busy="accountsStore.deletingSelectedEmail"
      :close-on-interact-outside="false"
      danger
      @update:open="deleteOpen = $event"
      @confirm="handleDelete"
    />
    <EmailTagDialog
      v-if="accountsStore.selectedEmailAccountId && accountsStore.selectedEmailFolder"
      v-model:open="tagDialogOpen"
      :account-id="accountsStore.selectedEmailAccountId"
      :folder="accountsStore.selectedEmailFolder"
      :uid="accountsStore.selectedEmailUid"
      @saved="onEmailTagsSaved"
    />
  </Teleport>
</template>

<style scoped>
.detail-alert {
  margin: 12px;
}
.email-detail {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  color: var(--n-text-color);
  background: var(--n-body-color);
}
/* Refreshing the message already on screen must not move it: a 2px bar slides at the top
   of the pane. The old in-flow spinner block was ~116px tall (48px padding twice), so the
   whole message jumped down for as long as the fetch lasted. */
.detail-progress {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 2px;
  overflow: hidden;
  pointer-events: none;
  background: color-mix(in srgb, var(--n-primary-color) 22%, transparent);
}
.detail-progress::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  width: 35%;
  background: var(--n-primary-color);
  animation: progress-slide 1s ease-in-out infinite;
}
@keyframes progress-slide {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(300%);
  }
}
.scroll {
  flex: 1;
  min-height: 0;
}
.centerpad {
  display: flex;
  justify-content: center;
  padding: 48px 0;
}
.centerpad-empty {
  opacity: 0.4;
  text-align: center;
  padding: 2rem;
}
.empty {
  color: var(--color-text-3);
  font-size: 13px;
}

.detail-body {
  padding: 18px 22px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding-top: 8px;
}
.detail-tags__button {
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
}
.detail-tags__button:hover {
  opacity: 0.8;
}
</style>
