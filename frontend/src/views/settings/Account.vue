<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from '../../i18n'
import { useAccountsStore } from '../../stores/accounts'
import {
  NCard,
  NButton,
  NButtonGroup,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSwitch,
  NSelect,
  NEmpty,
  NSpace,
  NDivider,
  NList,
  NListItem,
  NTag,
  NPopconfirm,
  NAlert,
  useMessage,
  useDialog,
  type FormInst,
  type FormRules,
  type SelectOption,
} from 'naive-ui'
import { EMAIL_PROVIDER_DEFAULTS, EMAIL_PROVIDER_LABELS, detectProvider } from '../../types/email'
import type { EmailAccount, EmailProvider } from '../../types/email'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const message = useMessage()
const dialog = useDialog()

const showingForm = ref(false)
const editingId = ref<string | null>(null)
const saving = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)
const formRef = ref<FormInst | null>(null)

// Per-item test tracking (matching LLM pattern)
const testingIndex = ref<string | null>(null)
const testSuccessIndex = ref<string | null>(null)
const testFailIndex = ref<string | null>(null)

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
})

const formData = ref<EmailAccount>(emptyForm())

const providerOptions = computed<SelectOption[]>(() =>
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
      const defaults = EMAIL_PROVIDER_DEFAULTS[provider]
      if (provider !== 'custom') {
        formData.value.IMAPHost = defaults.imap_host
        formData.value.IMAPPort = defaults.imap_port
        formData.value.UseTLS = defaults.imap_use_tls
        formData.value.SMTPHost = defaults.smtp_host
        formData.value.SMTPPort = defaults.smtp_port
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
      const defaults = EMAIL_PROVIDER_DEFAULTS[provider as EmailProvider]
      formData.value.IMAPHost = defaults.imap_host
      formData.value.IMAPPort = defaults.imap_port
      formData.value.UseTLS = defaults.imap_use_tls
      formData.value.SMTPHost = defaults.smtp_host
      formData.value.SMTPPort = defaults.smtp_port
    }
  },
)

const formRules: FormRules = {
  Name: [{ required: true, message: 'Please enter account name', trigger: 'blur' }],
  Email: [{ required: true, message: 'Please enter email address', trigger: 'blur' }],
  IMAPHost: [{ required: true, message: 'Please enter IMAP server', trigger: 'blur' }],
  IMAPPort: [
    { type: 'number', required: true, message: 'Please enter IMAP port', trigger: 'blur' },
  ],
  IMAPPass: [{ required: true, message: 'Please enter IMAP password', trigger: 'blur' }],
  SMTPHost: [{ required: true, message: 'Please enter SMTP server', trigger: 'blur' }],
  SMTPPort: [
    { type: 'number', required: true, message: 'Please enter SMTP port', trigger: 'blur' },
  ],
}

function openAddForm() {
  editingId.value = null
  formData.value = emptyForm()
  testResult.value = null
  showingForm.value = true
}

function openEditForm(account: EmailAccount) {
  editingId.value = account.ID
  formData.value = { ...account }
  testResult.value = null
  showingForm.value = true
}

function cancelForm() {
  showingForm.value = false
  editingId.value = null
  formData.value = emptyForm()
  testResult.value = null
}

async function handleSave() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await accountsStore.updateAccount(editingId.value, formData.value)
      message.success('Account updated')
    } else {
      const id =
        crypto.randomUUID?.() ?? Date.now().toString(36) + Math.random().toString(36).slice(2)
      await accountsStore.addAccount({ ...formData.value, ID: id })
      message.success('Account added')
    }
    cancelForm()
  } catch (e) {
    message.error(String(e))
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testResult.value = null
  const result = await accountsStore.testConnection(formData.value)
  testResult.value = result
  if (result.success) {
    message.success(result.message)
  } else {
    message.error(result.message)
  }
}

function handleDelete(account: EmailAccount) {
  dialog.warning({
    title: t.value['settings.accounts.delete'],
    content: `${t.value['settings.accounts.delete_confirm']} (${account.Email})`,
    positiveText: t.value['settings.accounts.delete'],
    negativeText: t.value['settings.accounts.cancel'],
    onPositiveClick: async () => {
      try {
        await accountsStore.removeAccount(account.ID)
        message.success('Account deleted')
      } catch (e) {
        message.error(String(e))
      }
    },
  })
}

async function handleTestItem(account: EmailAccount) {
  testingIndex.value = account.ID
  testSuccessIndex.value = null
  testFailIndex.value = null
  try {
    const result = await accountsStore.testConnection(account)
    if (result.success) {
      testSuccessIndex.value = account.ID
      message.success(result.message)
    } else {
      testFailIndex.value = account.ID
      message.error(result.message)
    }
    setTimeout(() => {
      if (testSuccessIndex.value === account.ID) testSuccessIndex.value = null
      if (testFailIndex.value === account.ID) testFailIndex.value = null
    }, 3000)
  } catch (e) {
    testFailIndex.value = account.ID
    message.error(String(e))
    setTimeout(() => {
      if (testFailIndex.value === account.ID) testFailIndex.value = null
    }, 3000)
  } finally {
    testingIndex.value = null
  }
}

function getProviderTag(provider: string): {
  type: 'default' | 'info' | 'warning' | 'error' | 'success' | 'primary'
} {
  switch (provider) {
    case 'gmail':
      return { type: 'success' }
    case 'outlook':
      return { type: 'info' }
    case 'qq':
      return { type: 'warning' }
    case '163':
      return { type: 'error' }
    default:
      return { type: 'warning' }
  }
}
</script>

<template>
  <div style="padding: 16px; max-width: 720px">
    <NCard :title="formTitle" size="small" v-if="showingForm">
      <NForm
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-placement="left"
        label-width="140"
        require-mark-placement="right"
      >
        <NFormItem :label="t['settings.accounts.name']" path="Name">
          <NInput v-model:value="formData.Name" placeholder="My Email" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.email']" path="Email">
          <NInput v-model:value="formData.Email" placeholder="user@example.com" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.provider']" path="Provider">
          <NSelect v-model:value="formData.Provider" :options="providerOptions" />
        </NFormItem>

        <NDivider>IMAP</NDivider>

        <NFormItem :label="t['settings.accounts.imap_host']" path="IMAPHost">
          <NInput v-model:value="formData.IMAPHost" placeholder="imap.example.com" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.imap_port']" path="IMAPPort">
          <NInputNumber
            v-model:value="formData.IMAPPort"
            :min="1"
            :max="65535"
            style="width: 120px"
          />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.imap_user']" path="IMAPUser">
          <NInput v-model:value="formData.IMAPUser" placeholder="user@example.com" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.imap_pass']" path="IMAPPass">
          <NInput v-model:value="formData.IMAPPass" type="password" show-password-on="click" />
        </NFormItem>

        <NAlert v-if="is163Provider" type="warning" closable style="margin-bottom: 8px">
          163 邮箱对第三方客户端有严格限制，即使使用授权码也可能连接失败。
          如果无法使用，建议改用官方客户端或 Foxmail。
        </NAlert>
        <NAlert v-else-if="needsAuthCode" type="warning" closable style="margin-bottom: 8px">
          {{ t['settings.accounts.auth_code_hint'] }}
        </NAlert>

        <NDivider>SMTP</NDivider>

        <NFormItem :label="t['settings.accounts.smtp_host']" path="SMTPHost">
          <NInput v-model:value="formData.SMTPHost" placeholder="smtp.example.com" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.smtp_port']" path="SMTPPort">
          <NInputNumber
            v-model:value="formData.SMTPPort"
            :min="1"
            :max="65535"
            style="width: 120px"
          />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.smtp_user']" path="SMTPUser">
          <NInput v-model:value="formData.SMTPUser" placeholder="user@example.com" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.smtp_pass']" path="SMTPPass">
          <NInput v-model:value="formData.SMTPPass" type="password" show-password-on="click" />
        </NFormItem>
        <NFormItem :label="t['settings.accounts.use_tls']" path="UseTLS">
          <NSwitch v-model:value="formData.UseTLS" />
        </NFormItem>

        <NDivider>{{ t['settings.accounts.sync'] }}</NDivider>

        <NFormItem :label="t['settings.accounts.auto_sync']" path="SyncInterval">
          <div style="display: flex; align-items: center; gap: 8px">
            <NSwitch
              :value="formData.SyncInterval > 0"
              @update:value="(val: boolean) => { formData.SyncInterval = val ? 5 : 0 }"
            />
            <template v-if="formData.SyncInterval > 0">
              <NInputNumber
                v-model:value="formData.SyncInterval"
                :min="1"
                :max="1440"
                style="width: 100px"
              />
              <span style="opacity: 0.6; font-size: 13px">{{ t['settings.accounts.sync_minutes'] }}</span>
            </template>
            <span v-else style="opacity: 0.4; font-size: 13px">{{ t['settings.accounts.sync_disabled'] }}</span>
          </div>
        </NFormItem>

        <NAlert
          v-if="testResult"
          :type="testResult.success ? 'success' : 'error'"
          closable
          style="margin-top: 12px"
        >
          {{ testResult.message }}
        </NAlert>

        <NSpace justify="end" style="margin-top: 16px">
          <NButton @click="cancelForm">{{ t['settings.accounts.cancel'] }}</NButton>
          <NButton :loading="accountsStore.testingId !== null" @click="handleTest">
            {{ t['settings.accounts.test'] }}
          </NButton>
          <NButton type="primary" :loading="saving" @click="handleSave">
            {{ t['settings.accounts.save'] }}
          </NButton>
        </NSpace>
      </NForm>
    </NCard>

    <NCard v-else :title="t['settings.accounts']" size="small">
      <template #header-extra>
        <NButton type="primary" size="small" @click="openAddForm">
          {{ t['settings.accounts.add'] }}
        </NButton>
      </template>

      <NEmpty
        v-if="accountsStore.accounts.length === 0"
        :description="t['settings.accounts.empty']"
      />

      <NList v-else bordered>
        <NListItem v-for="account in accountsStore.accounts" :key="account.ID">
          <template #default>
            <div style="display: flex; align-items: center; justify-content: space-between">
              <div style="display: flex; align-items: center; gap: 10px">
                <div class="avatar-circle">
                  {{ account.Email.charAt(0).toUpperCase() }}
                </div>
                <div>
                  <strong>{{ account.Name || account.Email }}</strong>
                  <NTag
                    :type="getProviderTag(account.Provider).type"
                    size="small"
                    style="margin-left: 8px"
                  >
                    {{
                      EMAIL_PROVIDER_LABELS[account.Provider as EmailProvider] || account.Provider
                    }}
                  </NTag>
                  <div style="font-size: 12px; opacity: 0.6; margin-top: 4px">
                    {{ account.Email }} &middot; IMAP: {{ account.IMAPHost }}:{{ account.IMAPPort }}
                  </div>
                </div>
              </div>
              <NButtonGroup>
                <NButton
                  size="small"
                  :loading="testingIndex === account.ID"
                  :disabled="testingIndex === account.ID"
                  :type="
                    testSuccessIndex === account.ID
                      ? 'success'
                      : testFailIndex === account.ID
                        ? 'error'
                        : 'default'
                  "
                  @click="handleTestItem(account)"
                >
                  {{ t['settings.accounts.test'] }}
                </NButton>
                <NButton size="small" @click="openEditForm(account)">
                  {{ t['settings.llm.edit'] }}
                </NButton>
                <NPopconfirm
                  :positive-text="t['settings.accounts.delete']"
                  :negative-text="t['settings.accounts.cancel']"
                  @positive-click="handleDelete(account)"
                >
                  <template #trigger>
                    <NButton size="small" type="error">
                      {{ t['settings.accounts.delete'] }}
                    </NButton>
                  </template>
                  {{ t['settings.accounts.delete_confirm'] }}
                </NPopconfirm>
              </NButtonGroup>
            </div>
          </template>
        </NListItem>
      </NList>
    </NCard>
  </div>
</template>

<style scoped>
.avatar-circle {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 600;
  font-size: 14px;
  color: #fff;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  flex-shrink: 0;
}
</style>
