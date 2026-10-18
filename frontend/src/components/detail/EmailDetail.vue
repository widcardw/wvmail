<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useThemeStore } from '~/stores/theme'
import { useFeedbackStore } from '~/stores/feedback'
import { usePreferencesStore } from '~/stores/preferences'
import { useLocaleStore } from '~/stores/locale'
import { formatDate } from '~/composables/formats'
import { sanitizeEmailHtml, linkifyPlainText } from '~/composables/html'
import { useMailTranslation } from '~/composables/mail-translation'
import EmailShadowBody from './EmailShadowBody.vue'
import EmailHeader from './EmailHeader.vue'
import EmailTranslatedBody from './EmailTranslatedBody.vue'
import EmailAttachments from './EmailAttachments.vue'
import EmailRemoteNotice from './EmailRemoteNotice.vue'
import EmailCssArtNotice from './EmailCssArtNotice.vue'
import EmailSecurityNotice from './EmailSecurityNotice.vue'
import EmailLinkDialog from './EmailLinkDialog.vue'
import RemindFromMailDialog from './RemindFromMailDialog.vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const themeStore = useThemeStore()
const fb = useFeedbackStore()
const preferences = usePreferencesStore()
const localeStore = useLocaleStore()

const isDarkMode = computed(() => themeStore.effectiveTheme === 'dark')
const detail = computed(() => accountsStore.selectedEmailDetail)
const loading = computed(() => accountsStore.loadingDetail)

const suspiciousRevealed = ref(false)
const attachmentsOpen = ref(false)
/** Revealed for this message only; reset whenever another email is opened. */
const remoteRevealed = ref(false)

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
  const html = detail.value?.body_html
  if (!html) return { html: '', blockedRemote: 0, cssArt: 0 }
  return sanitizeEmailHtml(html, sanitizeOptions())
})
const sanitizedBodyHtml = computed(() => sanitized.value.html)
const blockedRemote = computed(() => sanitized.value.blockedRemote)
/** Style declarations dropped because they were spam-grade (a picture drawn with CSS). */
const cssArt = computed(() => sanitized.value.cssArt)

/** Text-only messages: same URL + date highlighting, without the HTML pipeline. */
const plainBodyHtml = computed(() => {
  const text = detail.value?.body_text
  if (!text) return ''
  return linkifyPlainText(text, sanitizeOptions().mentions)
})

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

// Translation state machine (spam-gated, with the engine/model from Settings); the
// reading pane only wires it to the header button and the result panels.
const {
  translating,
  translatedText,
  translatedHtml,
  error: translationError,
  hasTranslation,
  hasOutput: hasTranslationOutput,
  clear: clearTranslation,
  toggle: toggleTranslation,
} = useMailTranslation({
  detail,
  sanitize: (html) => sanitizeEmailHtml(html, sanitizeOptions()).html,
  blocked: () => Boolean(detail.value?.is_suspicious) && !suspiciousRevealed.value,
})

watch(detail, () => {
  clearTranslation()
  suspiciousRevealed.value = false
  remoteRevealed.value = false
  attachmentsOpen.value = false
  remindOpen.value = false
})

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
  // Drop the stale translation so a re-fetch is always actionable.
  clearTranslation()
  const account = accountsStore.selectedAccount
  const folder = accountsStore.selectedFolder
  const uid = accountsStore.selectedEmailUid
  if (!account || !folder || uid == null) {
    fb.info(t.value['mail.refresh.aggregate'])
    return
  }
  await accountsStore.refreshEmailBody(account, folder, uid)
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
  if (href.startsWith('mailto:')) return
  if (href.startsWith('#') && !href.startsWith('http')) return

  event.preventDefault()
  pendingUrl.value = href.startsWith('http') ? href : el.href
  linkOpen.value = true
}

function closeError() {
  accountsStore.detailError = null
}

const showAttachments = computed(
  () =>
    detail.value?.attachments &&
    detail.value.attachments.length > 0 &&
    (!detail.value.is_suspicious || suspiciousRevealed.value),
)
</script>

<template>
  <div class="email-detail">
    <div class="scroll">
      <!-- fetch / translation errors (the spam gate lives in the body-bar stack below) -->
      <div v-if="accountsStore.detailError" class="alert alert--err with-x">
        <span>{{ accountsStore.detailError }}</span>
        <button type="button" class="x" aria-label="close" @click="closeError()">
          <i class="i-ri:close-line" />
        </button>
      </div>

      <div v-if="!detail && !loading" class="centerpad empty">{{ t['mail.selectFolder'] }}</div>

      <template v-if="detail">
        <EmailHeader
          :detail="detail"
          :loading="loading"
          :translating="translating"
          :has-translation="hasTranslation"
          @refresh="handleRefresh"
          @toggle-translation="toggleTranslation"
        />

        <!-- One bar stack above the body: the spam gate, then what the sanitiser dropped. -->
        <EmailSecurityNotice
          v-if="detail.is_suspicious && !suspiciousRevealed"
          @reveal="suspiciousRevealed = true"
        />

        <EmailCssArtNotice
          v-if="cssArt > 0 && (!detail.is_suspicious || suspiciousRevealed)"
          :count="cssArt"
        />

        <EmailRemoteNotice
          v-if="blockedRemote > 0 && (!detail.is_suspicious || suspiciousRevealed)"
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
            v-if="sanitizedBodyHtml && (!detail.is_suspicious || suspiciousRevealed)"
            :html="sanitizedBodyHtml"
            :dark="isDarkMode"
          />
          <EmailShadowBody v-else-if="plainBodyHtml" :html="plainBodyHtml" :dark="isDarkMode" />
          <div v-else class="centerpad-empty">{{ t['mail.noContent'] }}</div>
        </div>

        <EmailTranslatedBody
          v-if="hasTranslationOutput && (!detail.is_suspicious || suspiciousRevealed)"
          :html="translatedHtml"
          :text="translatedText"
          :error="translationError"
          :dark="isDarkMode"
          @dismiss-error="translationError = null"
        />
      </template>
    </div>

    <div v-if="loading" class="detail-progress" aria-hidden="true" />
  </div>
  <Teleport to="body">
    <EmailLinkDialog v-model:open="linkOpen" :url="pendingUrl" />
    <RemindFromMailDialog
      v-model:open="remindOpen"
      :at="remindAt"
      :text="remindText"
      :zone="remindZone"
      :suggested-title="detail?.subject || ''"
    />
  </Teleport>
</template>

<style scoped>
.email-detail {
  position: relative;
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
  height: 100%;
  overflow-y: auto;
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

.alert {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin: 12px;
  padding: 9px 12px;
  font-size: 13px;
  border-radius: 6px;
  border: 1px solid var(--n-border-color);
}
.alert--err {
  background: var(--color-danger-bg);
  border-color: var(--color-danger);
}
.x {
  border: none;
  background: none;
  cursor: pointer;
  color: inherit;
  opacity: 0.6;
}
.x i {
  font-size: 15px;
}
.with-x {
  align-items: center;
}

.detail-body {
  padding: 18px 22px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
</style>
