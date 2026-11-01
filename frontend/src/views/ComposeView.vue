<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Events } from '@wailsio/runtime'
import { useI18n } from '~/i18n'
import { ComposeService, ConfigService } from '#/wvmail/services'
import type { ComposeAttachment, ComposeDraft, EmailAccount } from '#/wvmail/services'
import ComposeEditor from '~/components/compose/ComposeEditor.vue'
import BusyButton from '~/components/ui/BusyButton.vue'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'
import UiSelect from '~/components/ui/UiSelect.vue'
import {
  formatRecipientText,
  isImageFilename,
  parseRecipientText,
} from '~/composables/compose'

const route = useRoute()
const { t, tf } = useI18n()

const draftID = ref(String(route.query.id ?? ''))
const accounts = ref<EmailAccount[]>([])
const accountID = ref('')
const toText = ref('')
const ccText = ref('')
const bccText = ref('')
const subject = ref('')
const html = ref('')
const text = ref('')
const attachments = ref<ComposeAttachment[]>([])
const inlineImages = ref<ComposeAttachment[]>([])
const showCc = ref(false)
const showBcc = ref(false)
const loading = ref(true)
const sending = ref(false)
const error = ref('')
const dirty = ref(false)
const closePrompt = ref(false)
const editorRef = ref<InstanceType<typeof ComposeEditor> | null>(null)
const original = ref<ComposeDraft | null>(null)
let hydrating = true
let saveTimer: ReturnType<typeof setTimeout> | null = null
let unlistenDrop: (() => void) | null = null
let unlistenClose: (() => void) | null = null

const accountOptions = computed(() =>
  accounts.value.map((account) => ({
    label: account.Name ? `${account.Name} <${account.Email}>` : account.Email,
    value: account.ID,
  })),
)

const canSend = computed(
  () =>
    Boolean(accountID.value) &&
    (parseRecipientText(toText.value).length > 0 ||
      parseRecipientText(ccText.value).length > 0 ||
      parseRecipientText(bccText.value).length > 0),
)

function setDirty() {
  if (hydrating) return
  dirty.value = true
  void ComposeService.SetDirty(draftID.value, true).catch(() => {})
  if (saveTimer !== null) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => void persistDraft(), 450)
}

function currentDraft(): ComposeDraft {
  return {
    ID: draftID.value,
    AccountID: accountID.value,
    To: parseRecipientText(toText.value),
    Cc: parseRecipientText(ccText.value),
    Bcc: parseRecipientText(bccText.value),
    Subject: subject.value,
    Text: text.value,
    HTML: html.value,
    Attachments: attachments.value,
    InlineImages: inlineImages.value,
    InReplyTo: original.value?.InReplyTo ?? '',
    References: original.value?.References ?? [],
  } as ComposeDraft
}

async function persistDraft() {
  if (saveTimer !== null) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  if (!draftID.value || hydrating) return
  await ComposeService.UpdateDraft(currentDraft())
}

watch([accountID, toText, ccText, bccText, subject, html, text, attachments, inlineImages], setDirty, {
  deep: true,
})

async function attachFiles() {
  try {
    const added = await ComposeService.PickAttachments(draftID.value)
    if (added.length === 0) return
    attachments.value.push(...added)
  } catch (e) {
    if (String(e).toLowerCase().includes('cancel')) return
    error.value = String(e)
  }
}

async function attachInlineImage() {
  try {
    const added = await ComposeService.PickInlineImages(draftID.value)
    if (added.length === 0) return
    inlineImages.value.push(...added)
    for (const image of added) insertImage(image)
  } catch (e) {
    if (String(e).toLowerCase().includes('cancel')) return
    error.value = String(e)
  }
}

function insertImage(image: ComposeAttachment) {
  if (!image.Preview) return
  editorRef.value?.insertImage({
    id: image.ID,
    preview: image.Preview,
    name: image.Name,
    contentId: image.ContentID ?? '',
  })
}

function removeAttachment(image: ComposeAttachment, inline: boolean) {
  if (inline) inlineImages.value = inlineImages.value.filter((item) => item.ID !== image.ID)
  else attachments.value = attachments.value.filter((item) => item.ID !== image.ID)
}

function formatSize(size: number): string {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

async function send() {
  if (sending.value) return
  error.value = ''
  if (!canSend.value) {
    error.value = t.value['compose.recipient_required']
    return
  }
  sending.value = true
  try {
    await persistDraft()
    await ComposeService.SendDraft(draftID.value)
  } catch (e) {
    error.value = String(e)
  } finally {
    sending.value = false
  }
}

async function discard() {
  closePrompt.value = false
  await ComposeService.CloseCompose(draftID.value).catch(() => {})
}

function requestClose() {
  closePrompt.value = true
}

async function onFilesDropped(event: { data?: unknown }) {
  const payload = event.data as
    | { draftID?: string; files?: string[]; targetID?: string }
    | undefined
  if (!payload || payload.draftID !== draftID.value || !payload.files?.length) return
  const inline =
    payload.targetID === 'compose-editor-drop' && payload.files.every((file) => isImageFilename(file))
  try {
    const added = await ComposeService.ImportFiles(draftID.value, payload.files, inline)
    if (inline) {
      inlineImages.value.push(...added)
      for (const image of added) insertImage(image)
    } else {
      attachments.value.push(...added)
    }
  } catch (e) {
    error.value = String(e)
  }
}

onMounted(async () => {
  try {
    const [draft, loadedAccounts] = await Promise.all([
      ComposeService.GetDraft(draftID.value),
      ConfigService.GetAccounts(),
    ])
    original.value = draft
    accounts.value = loadedAccounts
    accountID.value = draft.AccountID || loadedAccounts[0]?.ID || ''
    toText.value = formatRecipientText(draft.To)
    ccText.value = formatRecipientText(draft.Cc)
    bccText.value = formatRecipientText(draft.Bcc)
    subject.value = draft.Subject
    html.value = draft.HTML
    text.value = draft.Text
    attachments.value = [...draft.Attachments]
    inlineImages.value = [...draft.InlineImages]
    showCc.value = ccText.value.length > 0
    showBcc.value = bccText.value.length > 0
    await nextTick()
  } catch (e) {
    error.value = String(e)
  } finally {
    hydrating = false
    loading.value = false
  }

  unlistenDrop = Events.On('compose:files-dropped', onFilesDropped)
  unlistenClose = Events.On('compose:confirm-close', (event) => {
    if (event.data === draftID.value) requestClose()
  })
})

onBeforeUnmount(() => {
  unlistenDrop?.()
  unlistenClose?.()
  if (saveTimer !== null) clearTimeout(saveTimer)
})
</script>

<template>
  <div class="compose-shell">
    <header class="compose-topbar">
      <div class="compose-heading">
        <i class="i-ri:quill-pen-line" aria-hidden="true" />
        <span>{{ t['compose.title'] }}</span>
      </div>
      <div class="compose-top-actions">
        <button type="button" class="btn btn--ghost" :disabled="sending" @click="requestClose">
          {{ t['compose.discard'] }}
        </button>
        <BusyButton
          class="btn btn--primary"
          :label="t['compose.send']"
          :busy="sending"
          :busy-label="t['compose.sending']"
          :disabled="loading || !canSend"
          @click="send"
        >
          <i class="i-ri:send-plane-2-line" aria-hidden="true" />
        </BusyButton>
      </div>
    </header>

    <div v-if="error" class="compose-alert">
      <span>{{ error }}</span>
      <button type="button" class="alert-close" :aria-label="t['cancel']" @click="error = ''">
        <i class="i-ri:close-line" aria-hidden="true" />
      </button>
    </div>

    <div v-if="loading" class="compose-loading">{{ t['mail.loading'] }}</div>
    <div v-else class="compose-form">
      <div class="compose-field compose-field--from">
        <label class="compose-label">{{ t['compose.from'] }}</label>
        <UiSelect
          v-model="accountID"
          :options="accountOptions"
          :placeholder="t['compose.from']"
          width="100%"
          :disabled="accounts.length < 2"
        />
      </div>

      <div class="compose-field compose-recipient-row">
        <label class="compose-label">{{ t['compose.to'] }}</label>
        <input
          v-model="toText"
          class="compose-input"
          :placeholder="t['compose.recipient_placeholder']"
          autocomplete="off"
        />
        <button v-if="!showCc" type="button" class="link-btn" @click="showCc = true">
          {{ t['compose.add_cc'] }}
        </button>
        <button v-if="!showBcc" type="button" class="link-btn" @click="showBcc = true">
          {{ t['compose.add_bcc'] }}
        </button>
      </div>

      <div v-if="showCc" class="compose-field compose-recipient-row">
        <label class="compose-label">{{ t['compose.cc'] }}</label>
        <input
          v-model="ccText"
          class="compose-input"
          :placeholder="t['compose.recipient_placeholder']"
          autocomplete="off"
        />
      </div>

      <div v-if="showBcc" class="compose-field compose-recipient-row">
        <label class="compose-label">{{ t['compose.bcc'] }}</label>
        <input
          v-model="bccText"
          class="compose-input"
          :placeholder="t['compose.recipient_placeholder']"
          autocomplete="off"
        />
      </div>

      <div class="compose-field">
        <label class="compose-label">{{ t['compose.subject'] }}</label>
        <input
          v-model="subject"
          class="compose-input"
          :placeholder="t['compose.subject_placeholder']"
          autocomplete="off"
        />
      </div>

      <div id="compose-editor-drop" class="compose-editor-wrap" data-file-drop-target>
        <ComposeEditor
          ref="editorRef"
          v-model="html"
          @update:text="text = $event"
          @pick-image="attachInlineImage"
        />
      </div>

      <div class="compose-attachments">
        <div class="attachment-actions">
          <button type="button" class="attach-btn" @click="attachFiles">
            <i class="i-ri:attachment-2" aria-hidden="true" />
            {{ t['compose.attach'] }}
          </button>
          <button type="button" class="attach-btn" @click="attachInlineImage">
            <i class="i-ri:image-add-line" aria-hidden="true" />
            {{ t['compose.insert_image'] }}
          </button>
          <span v-if="attachments.length + inlineImages.length" class="attachment-count">
            {{ tf('compose.attachment_count', { count: attachments.length + inlineImages.length }) }}
          </span>
        </div>

        <div v-if="attachments.length || inlineImages.length" class="attachment-list">
          <div
            v-for="item in [...inlineImages, ...attachments]"
            :key="item.ID"
            class="attachment-chip"
          >
            <img v-if="item.Preview" :src="item.Preview" :alt="item.Name" class="attachment-thumb" />
            <i v-else class="i-ri:file-3-line" aria-hidden="true" />
            <span class="attachment-name" :title="item.Name">{{ item.Name }}</span>
            <span class="attachment-size">{{ formatSize(item.Size) }}</span>
            <button
              type="button"
              class="attachment-remove"
              :aria-label="t['compose.remove_attachment']"
              @click="removeAttachment(item, item.Inline)"
            >
              <i class="i-ri:close-line" aria-hidden="true" />
            </button>
          </div>
        </div>
      </div>
    </div>

    <ConfirmDialog
      :open="closePrompt"
      :title="t['compose.close_title']"
      :description="t['compose.close_body']"
      :confirm-label="t['compose.discard']"
      danger
      @update:open="closePrompt = $event"
      @confirm="discard"
    />
  </div>
</template>

<style scoped>
.compose-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  min-height: 0;
  color: var(--n-text-color);
  background: var(--n-body-color);
}
.compose-topbar {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  height: 54px;
  padding: 0 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.compose-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 700;
}
.compose-heading i {
  font-size: 18px;
  color: var(--n-primary-color);
}
.compose-top-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.compose-top-actions .btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.compose-alert {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 10px 16px 0;
  padding: 8px 10px;
  color: #b42318;
  background: color-mix(in srgb, #f04438 12%, var(--n-body-color));
  border: 1px solid color-mix(in srgb, #f04438 32%, transparent);
  border-radius: 6px;
  font-size: 12px;
}
.compose-alert span {
  flex: 1;
  min-width: 0;
}
.alert-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  color: inherit;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.alert-close:hover {
  background: color-mix(in srgb, currentColor 12%, transparent);
}
.compose-loading {
  padding: 48px;
  color: var(--color-text-3);
  text-align: center;
}
.compose-form {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  padding: 12px 16px 16px;
}
.compose-field {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 36px;
  border-bottom: 1px solid var(--n-border-color);
}
.compose-field--from {
  padding-bottom: 4px;
}
.compose-label {
  flex: 0 0 54px;
  color: var(--color-text-2);
  font-size: 12px;
}
.compose-input {
  flex: 1;
  min-width: 0;
  height: 34px;
  padding: 0;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  outline: none;
  font: inherit;
  font-size: 13px;
}
.compose-input::placeholder {
  color: var(--color-text-3);
}
.compose-recipient-row {
  flex-wrap: wrap;
}
.link-btn {
  padding: 2px 4px;
  color: var(--n-primary-color);
  background: transparent;
  border: none;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}
.link-btn:hover {
  text-decoration: underline;
}
.compose-editor-wrap {
  display: flex;
  flex: 1;
  min-height: 0;
  margin-top: 8px;
  overflow: hidden;
  border: 1px solid var(--n-border-color);
  border-radius: 7px;
  background: var(--color-surface);
}
.compose-editor-wrap.file-drop-target-active {
  border-color: var(--n-primary-color);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--n-primary-color) 20%, transparent);
}
.compose-attachments {
  flex-shrink: 0;
  padding-top: 8px;
}
.attachment-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.attach-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 7px;
  color: var(--color-text-2);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 5px;
  cursor: pointer;
  font: inherit;
  font-size: 12px;
}
.attach-btn:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
  border-color: var(--n-border-color);
}
.attachment-count {
  margin-left: auto;
  color: var(--color-text-3);
  font-size: 11px;
}
.attachment-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  max-height: 88px;
  padding-top: 6px;
  overflow-y: auto;
}
.attachment-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  height: 28px;
  padding: 0 4px 0 7px;
  color: var(--color-text-2);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 5px;
  font-size: 12px;
}
.attachment-chip > i {
  flex-shrink: 0;
  font-size: 15px;
}
.attachment-thumb {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  border-radius: 3px;
  object-fit: cover;
}
.attachment-name {
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.attachment-size {
  color: var(--color-text-3);
  font-size: 10px;
}
.attachment-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  color: var(--color-text-3);
  background: transparent;
  border: none;
  border-radius: 3px;
  cursor: pointer;
}
.attachment-remove:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
</style>
