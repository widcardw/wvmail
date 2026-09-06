<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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
  showingList.value = false
}

function openEditForm(account: EmailAccount) {
  editingId.value = account.ID
  formData.value = { ...account }
  testResult.value = null
  avatarPreview.value = ''
  avatarFileData.value = null
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
  showingList.value = true
  editingId.value = null
  formData.value = emptyForm()
  testResult.value = null
  avatarPreview.value = ''
  avatarFileData.value = null
}

function onAvatarFile(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  avatarPreview.value = URL.createObjectURL(file)

  if (!file.type.startsWith('image/')) {
    fb.error('请选择图片文件')
    return
  }
  const ext = '.' + (file.name.split('.').pop()?.toLowerCase() || 'png')
  avatarFileExt.value = ext
  const reader = new FileReader()
  reader.onload = () => {
    avatarFileData.value = reader.result as ArrayBuffer
  }
  reader.readAsArrayBuffer(file)
}

function validateForm(): string[] {
  const f = formData.value
  const errs: string[] = []
  if (!f.Name.trim()) errs.push('account name')
  if (!f.Email.trim()) errs.push('email')
  if (!f.IMAPHost.trim()) errs.push('IMAP server')
  if (!f.IMAPPort) errs.push('IMAP port')
  if (!f.IMAPPass) errs.push('IMAP password')
  if (!f.SMTPHost.trim()) errs.push('SMTP server')
  if (!f.SMTPPort) errs.push('SMTP port')
  return errs
}

async function handleSave() {
  const errs = validateForm()
  if (errs.length > 0) {
    fb.error('缺少必填项: ' + errs.join(', '))
    return
  }
  saving.value = true
  try {
    let accountId = editingId.value
    if (accountId) {
      await accountsStore.updateAccount(accountId, formData.value)
      fb.success('Account updated')
    } else {
      const id =
        crypto.randomUUID?.() ?? Date.now().toString(36) + Math.random().toString(36).slice(2)
      accountId = id
      await accountsStore.addAccount({ ...formData.value, ID: id })
      fb.success('Account added')
    }
    if (accountId && avatarFileData.value) {
      try {
        const bytes = new Uint8Array(avatarFileData.value)
        await ConfigService.SaveAccountAvatar(accountId, Array.from(bytes), avatarFileExt.value)
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
    fb.success('Account deleted')
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
    <div v-if="showingForm">
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
              placeholder="My Email"
            />
          </div>
          <div class="frow">
            <label class="flabel">{{ t['settings.accounts.email'] }}</label>
            <input
              class="input"
              :value="formData.Email"
              @input="formData.Email = strVal($event)"
              placeholder="user@example.com"
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
              placeholder="imap.example.com"
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
              placeholder="user@example.com"
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
            邮箱对第三方客户端有严格限制，即使使用授权码也可能连接失败。如果无法使用，建议改用官方客户端或
            Foxmail。
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
              placeholder="smtp.example.com"
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
              placeholder="user@example.com"
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
    </div>

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
