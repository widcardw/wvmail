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

const { t } = useI18n()
const accountsStore = useAccountsStore()
const fb = useFeedbackStore()

const showingList = ref(true)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)
const confirmingDeleteId = ref<string | null>(null)
const testingId = ref<string | null>(null)
const testSuccessId = ref<string | null>(null)
const testFailId = ref<string | null>(null)

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
      .then((uri) => {
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
  if (!form.IMAPPass) errors.push(t.value['settings.accounts.imap_pass'])
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
      accountId = crypto.randomUUID?.() ?? Date.now().toString(36) + Math.random().toString(36).slice(2)
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
  } catch (error) {
    fb.error(String(error))
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
      :confirming-delete-id="confirmingDeleteId"
      @add="openAddForm"
      @edit="openEditForm"
      @test="testItem"
      @delete="deleteAccount"
    />
  </div>
</template>

<style scoped>
.wrap {
  max-width: 760px;
  padding: 16px;
  color: var(--n-text-color);
}
</style>
