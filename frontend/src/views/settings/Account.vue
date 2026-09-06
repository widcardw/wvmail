<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import { ImageCropper, useImageCropper } from '@ark-ui/vue/image-cropper'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useFeedbackStore } from '~/stores/feedback'
import UiSelect from '~/components/UiSelect.vue'
import { EMAIL_PROVIDER_DEFAULTS, EMAIL_PROVIDER_LABELS, detectProvider } from '~/types/email'
import type { EmailAccount, EmailProvider } from '~/types/email'
import { ConfigService } from '#/wvmail/services'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const fb = useFeedbackStore()

const showingList = ref(true) // false => modal form
const showingForm = computed(() => !showingList.value)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)
const confirmingDeleteId = ref<string | null>(null)

const testingId = ref<string | null>(null)
const testSuccessId = ref<string | null>(null)
const testFailId = ref<string | null>(null)

const emptyForm = (): EmailAccount => ({
  ID: '',
  Name: '',
  Email: '',
  Provider: 'gmail',
  IMAPHost: '',
  IMAPPort: 993,
  IMAPUser: '',
  IMAPPass: '',
  SMTPHost: '',
  SMTPPort: 587,
  SMTPUser: '',
  SMTPPass: '',
  UseTLS: true,
  SyncInterval: 0,
  Avatar: '',
})

// Avatar handling
const avatarPreview = ref('')
const avatarFileData = ref<ArrayBuffer | null>(null)
const avatarFileExt = ref('')
const cropDialogOpen = ref(false)
const cropImageSrc = ref('')
const cropImageAspect = ref(1)
const cropSourceUrl = ref<string | null>(null)
const avatarPreviewUrl = ref<string | null>(null)
const cropSaving = ref(false)
const imageCropper = useImageCropper({ aspectRatio: 1 })

const cropViewportStyle = computed(() => {
  const maxWidth = typeof window === 'undefined' ? 420 : Math.max(220, window.innerWidth - 100)
  const maxHeight = typeof window === 'undefined' ? 360 : Math.min(420, window.innerHeight * 0.58)
  const width = Math.min(420, maxWidth, maxHeight * cropImageAspect.value)
  const height = width / cropImageAspect.value
  return {
    width: `${width}px`,
    height: `${height}px`,
  }
})

const formData = ref<EmailAccount>(emptyForm())

const providerOptions = computed<Array<{ label: string; value: EmailProvider }>>(() =>
  (Object.keys(EMAIL_PROVIDER_LABELS) as EmailProvider[]).map((key) => ({
    label: EMAIL_PROVIDER_LABELS[key],
    value: key,
  })),
)

const isEditing = computed(() => editingId.value !== null)
const formTitle = computed(() =>
  isEditing.value ? t.value['settings.accounts.edit'] : t.value['settings.accounts.add'],
)
const needsAuthCode = computed(
  () => formData.value.Provider === '163' || formData.value.Provider === 'qq',
)
const is163Provider = computed(() => formData.value.Provider === '163')

watch(
  () => formData.value.Email,
  (email) => {
    if (email && !isEditing.value) {
      const provider = detectProvider(email)
      formData.value.Provider = provider
      const d = EMAIL_PROVIDER_DEFAULTS[provider]
      if (provider !== 'custom') {
        formData.value.IMAPHost = d.imap_host
        formData.value.IMAPPort = d.imap_port
        formData.value.UseTLS = d.imap_use_tls
        formData.value.SMTPHost = d.smtp_host
        formData.value.SMTPPort = d.smtp_port
      }
      formData.value.IMAPUser = email
      formData.value.SMTPUser = email
    }
  },
)

watch(
  () => formData.value.Provider,
  (provider) => {
    if (provider !== 'custom' && !isEditing.value) {
      const d = EMAIL_PROVIDER_DEFAULTS[provider as EmailProvider]
      formData.value.IMAPHost = d.imap_host
      formData.value.IMAPPort = d.imap_port
      formData.value.UseTLS = d.imap_use_tls
      formData.value.SMTPHost = d.smtp_host
      formData.value.SMTPPort = d.smtp_port
    }
  },
)

function openAddForm() {
  editingId.value = null
  formData.value = emptyForm()
  testResult.value = null
  avatarPreview.value = ''
  avatarFileData.value = null
  revokeAvatarPreviewUrl()
  closeCropDialog()
  showingList.value = false
}

function openEditForm(account: EmailAccount) {
  editingId.value = account.ID
  formData.value = { ...account }
  testResult.value = null
  avatarPreview.value = ''
  avatarFileData.value = null
  revokeAvatarPreviewUrl()
  closeCropDialog()
  showingList.value = false
  if (account.Avatar) {
    ConfigService.GetAccountAvatar(account.ID)
      .then((uri) => {
        if (uri) avatarPreview.value = uri
      })
      .catch(() => {})
  }
}

function cancelForm() {
  closeCropDialog()
  revokeAvatarPreviewUrl()
  showingList.value = true
  editingId.value = null
  formData.value = emptyForm()
  testResult.value = null
  avatarPreview.value = ''
  avatarFileData.value = null
}

function revokeAvatarPreviewUrl() {
  if (avatarPreviewUrl.value) {
    URL.revokeObjectURL(avatarPreviewUrl.value)
    avatarPreviewUrl.value = null
  }
}

function revokeCropSource() {
  if (cropSourceUrl.value) {
    URL.revokeObjectURL(cropSourceUrl.value)
    cropSourceUrl.value = null
  }
  cropImageSrc.value = ''
}

function closeCropDialog() {
  cropDialogOpen.value = false
  revokeCropSource()
}

function readImageAspectRatio(url: string): Promise<number> {
  return new Promise((resolve) => {
    const image = new Image()
    image.onload = () => resolve(image.naturalWidth / image.naturalHeight || 1)
    image.onerror = () => resolve(1)
    image.src = url
  })
}

async function onAvatarFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!file.type.startsWith('image/')) {
    fb.error(t.value['settings.accounts.invalid_image'])
    return
  }
  revokeCropSource()
  const sourceUrl = URL.createObjectURL(file)
  cropSourceUrl.value = sourceUrl
  cropImageSrc.value = sourceUrl
  cropImageAspect.value = await readImageAspectRatio(sourceUrl)
  if (cropSourceUrl.value !== sourceUrl) return
  cropDialogOpen.value = true
}

async function confirmCrop() {
  cropSaving.value = true
  try {
    const result = await imageCropper.value.getCroppedImage({
      output: 'blob',
      type: 'image/png',
    })
    if (!(result instanceof Blob)) {
      fb.error(t.value['settings.accounts.crop_failed'])
      return
    }
    avatarFileData.value = await result.arrayBuffer()
    avatarFileExt.value = '.png'
    revokeAvatarPreviewUrl()
    avatarPreviewUrl.value = URL.createObjectURL(result)
    avatarPreview.value = avatarPreviewUrl.value
    closeCropDialog()
  } catch (e) {
    console.error('Failed to crop avatar', e)
    fb.error(t.value['settings.accounts.crop_failed'])
  } finally {
    cropSaving.value = false
  }
}

watch(cropDialogOpen, (open) => {
  if (!open) revokeCropSource()
})

onUnmounted(() => {
  revokeCropSource()
  revokeAvatarPreviewUrl()
})

function validateForm(): string[] {
  const f = formData.value
  const errs: string[] = []
  if (!f.Name.trim()) errs.push(t.value['settings.accounts.name'])
  if (!f.Email.trim()) errs.push(t.value['settings.accounts.email'])
  if (!f.IMAPHost.trim()) errs.push(t.value['settings.accounts.imap_host'])
  if (!f.IMAPPort) errs.push(t.value['settings.accounts.imap_port'])
  if (!f.IMAPPass) errs.push(t.value['settings.accounts.imap_pass'])
  if (!f.SMTPHost.trim()) errs.push(t.value['settings.accounts.smtp_host'])
  if (!f.SMTPPort) errs.push(t.value['settings.accounts.smtp_port'])
  return errs
}

async function handleSave() {
  const errs = validateForm()
  if (errs.length > 0) {
    fb.error(`${t.value['settings.accounts.required_fields']}: ${errs.join(', ')}`)
    return
  }
  saving.value = true
  try {
    let accountId = editingId.value
    if (accountId) {
      await accountsStore.updateAccount(accountId, formData.value)
      fb.success(t.value['settings.accounts.updated'])
    } else {
      const id =
        crypto.randomUUID?.() ?? Date.now().toString(36) + Math.random().toString(36).slice(2)
      accountId = id
      await accountsStore.addAccount({ ...formData.value, ID: id })
      fb.success(t.value['settings.accounts.added'])
    }
    if (accountId && avatarFileData.value) {
      try {
        const bytes = new Uint8Array(avatarFileData.value)
        const filename = await ConfigService.SaveAccountAvatar(
          accountId,
          Array.from(bytes),
          avatarFileExt.value,
        )
        accountsStore.markAvatarUpdated(accountId, filename)
      } catch (e) {
        console.error('Failed to save avatar', e)
      }
    }
    cancelForm()
  } catch (e) {
    fb.error(String(e))
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testResult.value = null
  const res = await accountsStore.testConnection(formData.value)
  testResult.value = res
  if (res.success) fb.success(res.message)
  else fb.error(res.message)
}

async function testItem(account: EmailAccount) {
  testingId.value = account.ID
  testSuccessId.value = null
  testFailId.value = null
  try {
    const res = await accountsStore.testConnection(account)
    if (res.success) {
      testSuccessId.value = account.ID
      fb.success(res.message)
    } else {
      testFailId.value = account.ID
      fb.error(res.message)
    }
  } catch (e) {
    testFailId.value = account.ID
    fb.error(String(e))
  } finally {
    testingId.value = null
    setTimeout(() => {
      if (testSuccessId.value === account.ID) testSuccessId.value = null
      if (testFailId.value === account.ID) testFailId.value = null
    }, 3000)
  }
}

async function deleteAccount(account: EmailAccount) {
  if (confirmingDeleteId.value !== account.ID) {
    confirmingDeleteId.value = account.ID
    setTimeout(() => {
      if (confirmingDeleteId.value === account.ID) confirmingDeleteId.value = null
    }, 3000)
    return
  }
  confirmingDeleteId.value = null
  try {
    await accountsStore.removeAccount(account.ID)
    fb.success(t.value['settings.accounts.deleted'])
  } catch (e) {
    fb.error(String(e))
  }
}

function intVal(e: Event): number {
  return Number((e.target as HTMLInputElement).value) || 0
}
function strVal(e: Event): string {
  return (e.target as HTMLInputElement).value
}
function setProvider(v: string) {
  formData.value.Provider = v as EmailProvider
}
function providerLabel(p: string): string {
  return EMAIL_PROVIDER_LABELS[p as EmailProvider] || p
}
</script>

<template>
  <div class="wrap">
    <!-- ===== Account form ===== -->
    <template v-if="showingForm">
      <div class="card">
        <header class="card__head">
          <h2>{{ formTitle }}</h2>
        </header>
        <div class="card__body">
          <div class="frow">
            <span class="flabel">{{ t['settings.accounts.avatar'] }}</span>
            <div class="avatar-area">
              <label class="avatar-box">
                <img v-if="avatarPreview" :src="avatarPreview" class="avatar-box__img" alt="" />
                <i v-else class="i-ri:add-line" aria-hidden="true" />
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  class="avatar-box__input"
                  @change="onAvatarFile"
                />
              </label>
              <div class="avatar-hint">{{ t['settings.accounts.upload_avatar'] }}</div>
            </div>
          </div>

          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.name'] }}</label>
            <input
              class="input"
              :value="formData.Name"
              @input="formData.Name = strVal($event)"
              :placeholder="t['settings.accounts.placeholder_name']"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.email'] }}</label>
            <input
              class="input"
              :value="formData.Email"
              @input="formData.Email = strVal($event)"
              :placeholder="t['settings.accounts.placeholder_email']"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.provider'] }}</label>
            <UiSelect
              :options="providerOptions"
              :model-value="formData.Provider"
              width="230px"
              @update:model-value="setProvider"
            />
          </div>

          <div class="sec">IMAP</div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.imap_host'] }}</label>
            <input
              class="input"
              :value="formData.IMAPHost"
              @input="formData.IMAPHost = strVal($event)"
              :placeholder="t['settings.accounts.placeholder_imap_host']"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.imap_port'] }}</label>
            <input
              class="input input--num"
              type="number"
              :value="formData.IMAPPort"
              @change="formData.IMAPPort = intVal($event)"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.imap_user'] }}</label>
            <input
              class="input"
              :value="formData.IMAPUser"
              @input="formData.IMAPUser = strVal($event)"
              :placeholder="t['settings.accounts.placeholder_email']"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.imap_pass'] }}</label>
            <input
              class="input"
              type="password"
              :value="formData.IMAPPass"
              @input="formData.IMAPPass = strVal($event)"
            />
          </div>

          <div v-if="is163Provider" class="hint hint--warn">
            163
            {{ t['settings.accounts.imap_warning'] }}
          </div>
          <div v-else-if="needsAuthCode" class="hint hint--warn">
            {{ t['settings.accounts.auth_code_hint'] }}
          </div>

          <div class="sec">SMTP</div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.smtp_host'] }}</label>
            <input
              class="input"
              :value="formData.SMTPHost"
              @input="formData.SMTPHost = strVal($event)"
              :placeholder="t['settings.accounts.placeholder_smtp_host']"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.smtp_port'] }}</label>
            <input
              class="input input--num"
              type="number"
              :value="formData.SMTPPort"
              @change="formData.SMTPPort = intVal($event)"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.smtp_user'] }}</label>
            <input
              class="input"
              :value="formData.SMTPUser"
              @input="formData.SMTPUser = strVal($event)"
              :placeholder="t['settings.accounts.placeholder_email']"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.smtp_pass'] }}</label>
            <input
              class="input"
              type="password"
              :value="formData.SMTPPass"
              @input="formData.SMTPPass = strVal($event)"
            />
          </div>
          <div class="frow">
            <span class="flabel">{{ t['settings.accounts.use_tls'] }}</span>
            <button
              type="button"
              class="switch"
              :class="{ on: formData.UseTLS }"
              role="switch"
              :aria-checked="formData.UseTLS"
              @click="formData.UseTLS = !formData.UseTLS"
            >
              <span class="switch__thumb" />
            </button>
          </div>

          <div class="sec">{{ t['settings.accounts.sync'] }}</div>
          <div class="frow">
            <span class="flabel">{{ t['settings.accounts.auto_sync'] }}</span>
            <div class="sync-row">
              <button
                type="button"
                class="switch"
                :class="{ on: formData.SyncInterval > 0 }"
                role="switch"
                :aria-checked="formData.SyncInterval > 0"
                @click="formData.SyncInterval = formData.SyncInterval > 0 ? 0 : 5"
              >
                <span class="switch__thumb" />
              </button>
              <template v-if="formData.SyncInterval > 0">
                <input
                  class="input input--sm"
                  type="number"
                  :value="formData.SyncInterval"
                  @change="formData.SyncInterval = intVal($event)"
                />
                <span class="muted">{{ t['settings.accounts.sync_minutes'] }}</span>
              </template>
              <span v-else class="muted">{{ t['settings.accounts.sync_disabled'] }}</span>
            </div>
          </div>

          <div
            v-if="testResult"
            class="hint"
            :class="testResult.success ? 'hint--ok' : 'hint--err'"
          >
            {{ testResult.message }}
          </div>
        </div>
        <footer class="card__foot">
          <button type="button" class="btn" @click="cancelForm">
            {{ t['settings.accounts.cancel'] }}
          </button>
          <button
            type="button"
            class="btn"
            :disabled="accountsStore.testingId !== null"
            @click="handleTest"
          >
            {{ accountsStore.testingId !== null ? '…' : t['settings.accounts.test'] }}
          </button>
          <button type="button" class="btn btn--primary" :disabled="saving" @click="handleSave">
            {{ saving ? '…' : t['settings.accounts.save'] }}
          </button>
        </footer>
      </div>
    <Dialog.Root v-model:open="cropDialogOpen" lazy-mount unmount-on-exit>
      <Teleport to="body">
        <Dialog.Backdrop class="crop-dialog-backdrop" />
        <Dialog.Positioner class="crop-dialog-positioner">
          <Dialog.Content class="crop-dialog-content">
            <header class="crop-dialog__head">
              <div>
                <Dialog.Title class="crop-dialog__title">{{ t['settings.accounts.crop_avatar'] }}</Dialog.Title>
                <Dialog.Description class="crop-dialog__description">
                  {{ t['settings.accounts.crop_avatar_hint'] }}
                </Dialog.Description>
              </div>
              <Dialog.CloseTrigger
                type="button"
                class="crop-dialog__close"
                :aria-label="t['settings.accounts.close_crop']"
                @click="closeCropDialog"
              >
                <i class="i-ri:close-line" aria-hidden="true" />
              </Dialog.CloseTrigger>
            </header>

            <ImageCropper.RootProvider
              :value="imageCropper"
              class="avatar-cropper"
            >
              <ImageCropper.Viewport
                class="avatar-cropper__viewport"
                :style="cropViewportStyle"
              >
                <ImageCropper.Image
                  v-if="cropImageSrc"
                  :key="cropImageSrc"
                  class="avatar-cropper__image"
                  :src="cropImageSrc"
                  :alt="t['settings.accounts.crop_image_alt']"
                />
                <ImageCropper.Selection class="avatar-cropper__selection">
                  <ImageCropper.Handle
                    v-for="position in ImageCropper.handles"
                    :key="position"
                    class="avatar-cropper__handle"
                    :position="position"
                  >
                    <div />
                  </ImageCropper.Handle>
                  <ImageCropper.Grid class="avatar-cropper__grid" axis="horizontal" />
                  <ImageCropper.Grid class="avatar-cropper__grid" axis="vertical" />
                </ImageCropper.Selection>
              </ImageCropper.Viewport>
            </ImageCropper.RootProvider>

            <footer class="crop-dialog__foot">
              <button type="button" class="btn" @click="closeCropDialog">
                {{ t['settings.accounts.cancel'] }}
              </button>
              <button type="button" class="btn btn--primary" :disabled="cropSaving" @click="confirmCrop">
                {{ cropSaving ? '…' : t['settings.accounts.crop_avatar_done'] }}
              </button>
            </footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Teleport>
    </Dialog.Root>
    </template>

    <!-- ===== Account list ===== -->
    <div v-else class="card">
      <header class="card__head">
        <h2>{{ t['settings.accounts'] }}</h2>
        <button type="button" class="btn btn--primary btn--sm" @click="openAddForm">
          {{ t['settings.accounts.add'] }}
        </button>
      </header>
      <p v-if="accountsStore.accounts.length === 0" class="empty">
        {{ t['settings.accounts.empty'] }}
      </p>
      <ul v-else class="list">
        <li v-for="account in accountsStore.accounts" :key="account.ID" class="row">
          <div class="row-info">
            <strong>{{ account.Name || account.Email }}</strong>
            <span class="chip">{{ providerLabel(account.Provider) }}</span>
            <div class="muted">
              {{ account.Email }} &middot; IMAP: {{ account.IMAPHost }}:{{ account.IMAPPort }}
            </div>
          </div>
          <div class="row-actions">
            <button
              type="button"
              class="drv"
              :class="{
                'drv--ok': testSuccessId === account.ID,
                'drv--bad': testFailId === account.ID,
                disabled: testingId === account.ID,
              }"
              :disabled="testingId === account.ID"
              @click="testItem(account)"
            >
              {{ testingId === account.ID ? '…' : t['settings.accounts.test'] }}
            </button>
            <button type="button" class="drv" @click="openEditForm(account)">
              {{ t['settings.llm.edit'] }}
            </button>
            <button
              type="button"
              class="drv drv--danger"
              :disabled="testingId === account.ID"
              @click="deleteAccount(account)"
            >
              {{
                confirmingDeleteId === account.ID
                  ? t['settings.accounts.delete'] + '?'
                  : t['settings.accounts.delete']
              }}
            </button>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.wrap {
  padding: 16px;
  max-width: 760px;
  color: var(--n-text-color);
}
.card {
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
}
.card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.card__head h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.card__body {
  padding: 8px 16px;
}
.card__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 16px;
  border-top: 1px solid var(--n-border-color);
}
.frow {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 7px 0;
}
.input {
  flex: 1;
  min-width: 0;
}
.input--num {
  flex: 0 0 120px;
}
.input--sm {
  flex: 0 0 90px;
}
.sec {
  font-size: 13px;
  font-weight: 600;
  padding: 12px 0 4px;
  color: var(--color-text-2);
}
.hint {
  margin: 10px 0;
  padding: 8px 12px;
  font-size: 13px;
  color: var(--n-text-color);
  border-radius: 6px;
  border: 1px solid var(--n-border-color);
}
.hint--warn {
  background: rgba(240, 160, 32, 0.15);
  border-color: #f0a020;
}
.hint--ok {
  background: rgba(24, 160, 88, 0.12);
}
.hint--err {
  background: rgba(208, 48, 80, 0.12);
}
.sync-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.avatar-area {
  display: flex;
  align-items: center;
  gap: 12px;
}
.avatar-box {
  width: 64px;
  height: 64px;
  border-radius: 10px;
  border: 1px dashed var(--n-border-color);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  overflow: hidden;
  color: var(--color-text-3);
  background: var(--n-action-color);
}
.avatar-box i {
  font-size: 22px;
}
.avatar-box__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.avatar-box__input {
  display: none;
}
.avatar-hint {
  font-size: 12px;
  opacity: 0.6;
}
.crop-dialog-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgb(0 0 0 / 55%);
}
.crop-dialog-positioner {
  position: fixed;
  inset: 0;
  z-index: 1101;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}
.crop-dialog-content {
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(520px, calc(100vw - 32px));
  max-height: calc(100vh - 32px);
  overflow: auto;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 20px 50px rgb(0 0 0 / 30%);
}
.crop-dialog__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px 12px;
}
.crop-dialog__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.crop-dialog__description {
  margin-top: 5px;
  color: var(--color-text-2);
  font-size: 12px;
}
.crop-dialog__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  flex: 0 0 26px;
  padding: 0;
  color: var(--color-text-2);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.crop-dialog__close:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.crop-dialog__close:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.avatar-cropper {
  --cropper-accent: var(--n-primary-color);
  --cropper-line-color: rgb(255 255 255 / 50%);
  --cropper-overlay-color: rgb(0 0 0 / 50%);
  --cropper-handler-size: 6px;
  --cropper-handler-width: 3px;
  --cropper-line-width: 2px;

  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 0 18px;
  color: var(--n-text-color);
}
.avatar-cropper__viewport {
  position: relative;
  max-width: 100%;
  margin: 0 auto;
  overflow: hidden;
  background: var(--n-action-color);
  border-radius: 5px;
  touch-action: none;
}
.avatar-cropper__image {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  transform-origin: center center;
  pointer-events: none;
  user-select: none;
  backface-visibility: hidden;
}
.avatar-cropper__selection {
  box-sizing: content-box;
  border: var(--cropper-line-width) solid var(--cropper-line-color);
  box-shadow: 0 0 0 9999px var(--cropper-overlay-color);
  cursor: move;
  outline: none;
  backface-visibility: hidden;
  touch-action: none;
}
.avatar-cropper__selection:focus-visible {
  border-color: var(--cropper-accent);
}
.avatar-cropper__selection[data-dragging] {
  cursor: grabbing;
  border-color: rgb(255 255 255 / 80%);
}
.avatar-cropper__grid {
  position: absolute;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.2s ease;
}
.avatar-cropper__grid[data-axis='horizontal'] {
  inset: 33.33% 0;
  border-top: 1px solid rgb(255 255 255 / 40%);
  border-bottom: 1px solid rgb(255 255 255 / 40%);
}
.avatar-cropper__grid[data-axis='vertical'] {
  inset: 0 33.33%;
  border-right: 1px solid rgb(255 255 255 / 40%);
  border-left: 1px solid rgb(255 255 255 / 40%);
}
.avatar-cropper__grid[data-dragging],
.avatar-cropper__grid[data-panning] {
  opacity: 1;
}
.avatar-cropper__handle {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  width: calc(var(--cropper-handler-size) + 8px);
  height: calc(var(--cropper-handler-size) + 8px);
  touch-action: none;
}
.avatar-cropper__handle > div {
  width: var(--cropper-handler-size);
  height: var(--cropper-handler-size);
  background: var(--color-surface);
  box-shadow: 0 1px 3px rgb(0 0 0 / 30%);
  transition:
    opacity 0.2s ease,
    transform 0.15s ease;
}
.avatar-cropper__handle[data-disabled] {
  display: none;
}
.avatar-cropper__handle[data-position='top-left'],
.avatar-cropper__handle[data-position='top-right'],
.avatar-cropper__handle[data-position='bottom-right'],
.avatar-cropper__handle[data-position='bottom-left'] {
  cursor: nwse-resize;
}
.avatar-cropper__handle[data-position='top-left'] > div {
  border-top: var(--cropper-handler-width) solid var(--cropper-accent);
  border-left: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='top-right'] {
  cursor: nesw-resize;
}
.avatar-cropper__handle[data-position='top-right'] > div {
  border-top: var(--cropper-handler-width) solid var(--cropper-accent);
  border-right: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='bottom-right'] > div {
  border-right: var(--cropper-handler-width) solid var(--cropper-accent);
  border-bottom: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='bottom-left'] {
  cursor: nesw-resize;
}
.avatar-cropper__handle[data-position='bottom-left'] > div {
  border-bottom: var(--cropper-handler-width) solid var(--cropper-accent);
  border-left: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='top'] > div,
.avatar-cropper__handle[data-position='bottom'] > div,
.avatar-cropper__handle[data-position='left'] > div,
.avatar-cropper__handle[data-position='right'] > div {
  width: var(--cropper-handler-size);
  height: var(--cropper-handler-size);
  border: none;
  border-radius: 9999px;
  background: var(--cropper-accent);
  opacity: 0;
}
.avatar-cropper__handle[data-position='top']:hover > div,
.avatar-cropper__handle[data-position='bottom']:hover > div,
.avatar-cropper__handle[data-position='left']:hover > div,
.avatar-cropper__handle[data-position='right']:hover > div {
  opacity: 1;
}
.avatar-cropper__handle[data-position='top'],
.avatar-cropper__handle[data-position='bottom'] {
  cursor: ns-resize;
}
.avatar-cropper__handle[data-position='left'],
.avatar-cropper__handle[data-position='right'] {
  cursor: ew-resize;
}
.crop-dialog__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px 16px;
}
.empty-message + p,
.empty {
  padding: 30px 18px;
  color: var(--color-text-3);
  font-size: 13px;
}
.list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.row {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
  align-items: flex-start;
}
.row:last-child {
  border-bottom: none;
}
.row-info {
  min-width: 0;
}
.row-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}
.drv {
  border: 1px solid var(--n-border-color);
  background: var(--n-action-color);
  color: var(--n-text-color);
  border-radius: 5px;
  cursor: pointer;
  padding: 3px 8px;
  font-size: 12px;
}
.drv:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.drv--danger {
  color: #d03050;
}
.drv--ok {
  background: rgba(24, 160, 88, 0.2);
}
.drv--bad {
  background: rgba(208, 48, 80, 0.2);
}
</style>
