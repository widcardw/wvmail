<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useFeedbackStore } from '~/stores/feedback'
import { EMAIL_PROVIDER_LABELS } from '~/types/email'
import type { EmailAccount, EmailProvider } from '~/types/email'
import { ConfigService } from '#/wvmail/services'
import AccountForm from '~/components/settings/AccountForm.vue'
import AccountList from '~/components/settings/AccountList.vue'
import AvatarCropDialog from '~/components/settings/AvatarCropDialog.vue'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'

const { t, tf } = useI18n()
const accountsStore = useAccountsStore()
const fb = useFeedbackStore()

const showingList = ref(true)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)
const testingId = ref<string | null>(null)
const testSuccessId = ref<string | null>(null)
const testFailId = ref<string | null>(null)
/** The account the delete dialog is asking about; null keeps the dialog shut. */
const deleteTarget = ref<EmailAccount | null>(null)
/** Latched while RemoveAccount runs, so the dialog cannot be dismissed mid-write. */
const deleting = ref(false)
/**
 * The dialog's "also delete cached mail" tick. Off by default and reset on every
 * open: the cache is what makes the app readable offline, so discarding it is a
 * deliberate choice, never the thing you get by clicking through.
 */
const purgeCache = ref(false)

const avatarPreview = ref('')
const avatarFileData = ref<ArrayBuffer | null>(null)
const avatarFileExt = ref('')
const cropDialogOpen = ref(false)
const cropImageSrc = ref('')
const cropImageAspect = ref(1)
const cropSourceUrl = ref<string | null>(null)
const avatarPreviewUrl = ref<string | null>(null)

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
  AuthType: 'password',
  OAuthProvider: '',
  OAuthClientID: '',
  OAuthRefreshToken: '',
  SyncInterval: 0,
  Avatar: '',
})

const formData = ref<EmailAccount>(emptyForm())
const providerOptions = computed<Array<{ label: string; value: EmailProvider }>>(() =>
  (Object.keys(EMAIL_PROVIDER_LABELS) as EmailProvider[]).map((key) => ({
    label: EMAIL_PROVIDER_LABELS[key],
    value: key,
  })),
)
const isEditing = computed(() => editingId.value !== null)
const needsAuthCode = computed(
  () => formData.value.Provider === '163' || formData.value.Provider === 'qq',
)
const is163Provider = computed(() => formData.value.Provider === '163')

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

function onCropDialogOpen(open: boolean) {
  cropDialogOpen.value = open
  if (!open) revokeCropSource()
}

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
      .then((uri: string) => {
        if (uri && editingId.value === account.ID) avatarPreview.value = uri
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

function readImageAspectRatio(url: string): Promise<number> {
  return new Promise((resolve) => {
    const image = new Image()
    image.onload = () => resolve(image.naturalWidth / image.naturalHeight || 1)
    image.onerror = () => resolve(1)
    image.src = url
  })
}

async function onAvatarFile(event: Event) {
  const input = event.target as HTMLInputElement
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
  if (cropSourceUrl.value === sourceUrl) cropDialogOpen.value = true
}

async function onAvatarCropped(image: Blob) {
  avatarFileData.value = await image.arrayBuffer()
  avatarFileExt.value = '.png'
  revokeAvatarPreviewUrl()
  avatarPreviewUrl.value = URL.createObjectURL(image)
  avatarPreview.value = avatarPreviewUrl.value
  closeCropDialog()
}

function arrayBufferToBase64(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  const chunkSize = 0x8000
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize))
  }
  return btoa(binary)
}

function validateForm(): string[] {
  const form = formData.value
  const errors: string[] = []
  if (!form.Name.trim()) errors.push(t.value['settings.accounts.name'])
  if (!form.Email.trim()) errors.push(t.value['settings.accounts.email'])
  if (!form.IMAPHost.trim()) errors.push(t.value['settings.accounts.imap_host'])
  if (!form.IMAPPort) errors.push(t.value['settings.accounts.imap_port'])
  // OAuth accounts authenticate with a stored refresh token: there is no
  // password to require, but the verification itself must have succeeded.
  if (form.AuthType === 'oauth2') {
    if (!form.OAuthRefreshToken) errors.push(t.value['settings.accounts.oauth.not_verified'])
  } else if (!form.IMAPPass) {
    errors.push(t.value['settings.accounts.imap_pass'])
  }
  if (!form.SMTPHost.trim()) errors.push(t.value['settings.accounts.smtp_host'])
  if (!form.SMTPPort) errors.push(t.value['settings.accounts.smtp_port'])
  return errors
}

async function handleSave() {
  const errors = validateForm()
  if (errors.length > 0) {
    fb.error(`${t.value['settings.accounts.required_fields']}: ${errors.join(', ')}`)
    return
  }

  saving.value = true
  try {
    let accountId = editingId.value
    if (accountId) {
      await accountsStore.updateAccount(accountId, formData.value)
      fb.success(t.value['settings.accounts.updated'])
    } else {
      accountId =
        crypto.randomUUID?.() ?? Date.now().toString(36) + Math.random().toString(36).slice(2)
      await accountsStore.addAccount({ ...formData.value, ID: accountId })
      fb.success(t.value['settings.accounts.added'])
    }

    if (accountId && avatarFileData.value) {
      try {
        const filename = await ConfigService.SaveAccountAvatar(
          accountId,
          arrayBufferToBase64(avatarFileData.value),
          avatarFileExt.value,
        )
        accountsStore.markAvatarUpdated(accountId, filename)
      } catch (error) {
        console.error('Failed to save avatar', error)
      }
    }
    cancelForm()
  } catch (error) {
    fb.error(String(error))
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testResult.value = null
  const result = await accountsStore.testConnection(formData.value)
  testResult.value = result
  if (result.success) fb.success(result.message)
  else fb.error(result.message)
}

async function testItem(account: EmailAccount) {
  testingId.value = account.ID
  testSuccessId.value = null
  testFailId.value = null
  try {
    const result = await accountsStore.testConnection(account)
    if (result.success) {
      testSuccessId.value = account.ID
      fb.success(result.message)
    } else {
      testFailId.value = account.ID
      fb.error(result.message)
    }
  } catch (error) {
    testFailId.value = account.ID
    fb.error(String(error))
  } finally {
    testingId.value = null
    setTimeout(() => {
      if (testSuccessId.value === account.ID) testSuccessId.value = null
      if (testFailId.value === account.ID) testFailId.value = null
    }, 3000)
  }
}

/** Reorder one account by one slot; the store applies it optimistically. */
async function moveAccount(account: EmailAccount, offset: -1 | 1) {
  try {
    await accountsStore.moveAccount(account.ID, offset)
  } catch (error) {
    fb.error(String(error))
  }
}

/**
 * Deleting is destructive and irreversible, so it is asked about in a dialog
 * rather than by turning the button into a "delete?" label: the old two-click
 * trick was easy to misread, and it had no way to say *which* account it was
 * about to drop.
 */
function askDelete(account: EmailAccount) {
  purgeCache.value = false
  deleteTarget.value = account
}

const deleteDescription = computed(() => {
  const account = deleteTarget.value
  if (!account) return ''
  return tf('settings.accounts.delete_body', {
    name: account.Name || account.Email,
    email: account.Email,
  })
})

function onDeleteDialogOpen(open: boolean) {
  if (!open && !deleting.value) deleteTarget.value = null
}

async function confirmDelete() {
  const account = deleteTarget.value
  if (!account || deleting.value) return
  deleting.value = true
  try {
    await accountsStore.removeAccount(account.ID, { purgeCache: purgeCache.value })
    fb.success(t.value['settings.accounts.deleted'])
    deleteTarget.value = null
  } catch (error) {
    // Leave the dialog up: the account is still there, and the toast says why.
    fb.error(String(error))
  } finally {
    deleting.value = false
  }
}

onUnmounted(() => {
  revokeCropSource()
  revokeAvatarPreviewUrl()
})
</script>

<template>
  <div class="wrap">
    <template v-if="!showingList">
      <AccountForm
        v-model="formData"
        :editing="isEditing"
        :avatar-preview="avatarPreview"
        :provider-options="providerOptions"
        :needs-auth-code="needsAuthCode"
        :is163-provider="is163Provider"
        :test-result="testResult"
        :testing="accountsStore.testingId !== null"
        :saving="saving"
        @avatar-file="onAvatarFile"
        @cancel="cancelForm"
        @test="handleTest"
        @save="handleSave"
      />
      <AvatarCropDialog
        :open="cropDialogOpen"
        :image-src="cropImageSrc"
        :image-aspect="cropImageAspect"
        @update:open="onCropDialogOpen"
        @cropped="onAvatarCropped"
      />
    </template>

    <AccountList
      v-else
      :accounts="accountsStore.accounts"
      :testing-id="testingId"
      :test-success-id="testSuccessId"
      :test-fail-id="testFailId"
      @add="openAddForm"
      @edit="openEditForm"
      @test="testItem"
      @move="moveAccount"
      @delete="askDelete"
    />

    <ConfirmDialog
      :open="deleteTarget !== null"
      :title="t['settings.accounts.delete_title']"
      :description="deleteDescription"
      :confirm-label="t['settings.accounts.delete']"
      :busy="deleting"
      danger
      @update:open="onDeleteDialogOpen"
      @confirm="confirmDelete"
    >
      <template #extra>
        <UiCheckbox
          v-model:checked="purgeCache"
          class="cache-row"
          label-class="cache-row__label"
          :disabled="deleting"
        >
          {{ t['settings.accounts.delete_cache'] }}
        </UiCheckbox>
        <p class="cache-hint">{{ t['settings.accounts.delete_cache_hint'] }}</p>
      </template>
    </ConfirmDialog>
  </div>
</template>

<style scoped>
.wrap {
  max-width: 800px;
  padding: 16px;
  color: var(--n-text-color);
}

/* The delete dialog's optional extra step (see the #extra slot above). The
   checkbox row is an Ark <label>, so the whole line toggles it. */
.cache-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  cursor: pointer;
}
.cache-row__label {
  cursor: pointer;
}
/* Aligned under the label, past the 16px box + 8px gap. */
.cache-hint {
  margin: 2px 0 0 24px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--color-text-3);
}
</style>
