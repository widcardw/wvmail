<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { Field } from '@ark-ui/vue/field'
import { Tabs } from '@ark-ui/vue/tabs'
import BusyButton from '~/components/ui/BusyButton.vue'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiPasswordInput from '~/components/ui/UiPasswordInput.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import { EMAIL_PROVIDER_DEFAULTS, detectProvider } from '~/types/email'
import type { EmailAccount, EmailProvider } from '~/types/email'
import type { OAuthPollResult } from '#/wvmail/services'
import { OAuthService, OsService } from '#/wvmail/services'

const props = defineProps<{
  editing: boolean
  avatarPreview: string
  providerOptions: Array<{ label: string; value: EmailProvider }>
  needsAuthCode: boolean
  is163Provider: boolean
  testResult: { success: boolean; message: string } | null
  testing: boolean
  saving: boolean
}>()

const form = defineModel<EmailAccount>({ required: true })
const emit = defineEmits<{
  cancel: []
  test: []
  save: []
  avatarFile: [event: Event]
}>()

const { t } = useI18n()

// Two ways to add an account: type the server details by hand, or finish a
// browser-based OAuth round trip. The tab strip lives in the card header,
// right of the title (Ark Tabs, same look as the reminders page). Ark's
// Tabs.Root binds through v-model (modelValue), so `tab` stays a plain string.
// Adding defaults to the manual form; editing an OAuth account opens on the
// online-verification tab so a re-auth is one click away (the strip is hidden
// while editing and the pane follows the account's stored auth type).
const tab = ref<string>(props.editing && form.value.AuthType === 'oauth2' ? 'oauth' : 'manual')

const isOAuth = computed(() => form.value.AuthType === 'oauth2')
const isGmailManual = computed(() => tab.value === 'manual' && form.value.Provider === 'gmail')

/**
 * The note that applies to the IMAP password field, if any. Several providers
 * reject the account's login password and want an app-specific one instead;
 * that is a property of *this* input, so it belongs under the input as a
 * helper text rather than as a page-wide banner that shoves the form around.
 * Only one note can apply at a time, in the same precedence as before.
 */
const passwordHint = computed(() => {
  if (props.is163Provider) return `163 ${t.value['settings.accounts.imap_warning']}`
  if (props.needsAuthCode) return t.value['settings.accounts.auth_code_hint']
  if (isGmailManual.value) return t.value['settings.accounts.oauth.gmail_app_password_hint']
  return ''
})

// --- Online verification (OAuth) ---

type OAuthProviderId = 'google' | 'microsoft'
const oauthProvider = ref<OAuthProviderId>(
  form.value.OAuthProvider === 'google' ? 'google' : 'microsoft',
)
const oauthClientID = ref(form.value.OAuthClientID ?? '')
const oauthState = ref('')
const oauthWaiting = ref(false)
const oauthError = ref('')
const oauthResult = ref<OAuthPollResult | null>(null)

const oauthProviderOptions = computed(() => [
  { label: t.value['settings.accounts.oauth.microsoft'], value: 'microsoft' },
  { label: t.value['settings.accounts.oauth.google'], value: 'google' },
])

let pollTimer: ReturnType<typeof setInterval> | null = null

function stopPolling() {
  if (pollTimer !== null) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

async function startOAuth() {
  oauthError.value = ''
  oauthResult.value = null
  oauthWaiting.value = true
  try {
    const authURL = await OAuthService.BeginOAuth(oauthProvider.value, oauthClientID.value.trim())
    oauthState.value = await extractState(authURL)
    await OsService.OpenURL(authURL)
    // The loopback server answers the redirect; poll until the session is
    // consumed (success, denial, or the backend's 5-minute timeout).
    pollTimer = setInterval(pollOAuth, 1500)
  } catch (e) {
    oauthWaiting.value = false
    oauthError.value = String(e)
  }
}

/** The backend returns the authorization URL; the state rides in its query. */
async function extractState(authURL: string): Promise<string> {
  const state = new URL(authURL).searchParams.get('state')
  if (!state) throw new Error('authorization URL has no state')
  return state
}

async function pollOAuth() {
  if (!oauthState.value) return
  try {
    const res: OAuthPollResult = await OAuthService.PollOAuth(oauthState.value)
    if (!res.done) return
    stopPolling()
    oauthWaiting.value = false
    if (res.error || !res.email) {
      oauthError.value = res.error || t.value['settings.accounts.oauth.empty_email']
      return
    }
    applyOAuthResult(res)
  } catch (e) {
    stopPolling()
    oauthWaiting.value = false
    oauthError.value = String(e)
  }
}

/** Fill the account form from the verified identity and token. */
function applyOAuthResult(res: OAuthPollResult) {
  const email = res.email ?? ''
  const providerId = res.provider === 'google' ? 'google' : 'microsoft'
  const provider: EmailProvider = providerId === 'google' ? 'gmail' : 'outlook'
  form.value.Email = email
  form.value.IMAPUser = email
  form.value.SMTPUser = email
  form.value.Provider = provider
  const defaults = EMAIL_PROVIDER_DEFAULTS[provider]
  form.value.IMAPHost = defaults.imap_host
  form.value.IMAPPort = defaults.imap_port
  form.value.UseTLS = defaults.imap_use_tls
  form.value.SMTPHost = defaults.smtp_host
  form.value.SMTPPort = defaults.smtp_port
  form.value.AuthType = 'oauth2'
  form.value.OAuthProvider = providerId
  form.value.OAuthClientID = oauthClientID.value.trim()
  form.value.OAuthRefreshToken = res.refresh_token ?? ''
  form.value.IMAPPass = ''
  form.value.SMTPPass = ''
  oauthResult.value = res
}

onUnmounted(() => {
  stopPolling()
  // Abandoned sessions hold a loopback port; release them like the timeout would.
  if (oauthState.value && oauthWaiting.value) {
    OAuthService.CancelOAuth(oauthState.value).catch(() => {})
  }
})

// --- Manual (server settings) helpers, unchanged behaviour ---

function strVal(e: Event): string {
  return (e.target as HTMLInputElement).value
}

function setProvider(value: string) {
  form.value.Provider = value as EmailProvider
  if (!props.editing && value !== 'custom') {
    const defaults = EMAIL_PROVIDER_DEFAULTS[value as EmailProvider]
    form.value.IMAPHost = defaults.imap_host
    form.value.IMAPPort = defaults.imap_port
    form.value.UseTLS = defaults.imap_use_tls
    form.value.SMTPHost = defaults.smtp_host
    form.value.SMTPPort = defaults.smtp_port
  }
}

/** Auto-sync switch: 5 minute interval when on, 0 (disabled) when off. */
function onAutoSyncToggle(checked: boolean) {
  form.value.SyncInterval = checked ? 5 : 0
}

function onEmailInput(email: string) {
  form.value.Email = email
  if (email && !props.editing) {
    const provider = detectProvider(email)
    form.value.Provider = provider
    const defaults = EMAIL_PROVIDER_DEFAULTS[provider]
    if (provider !== 'custom') {
      form.value.IMAPHost = defaults.imap_host
      form.value.IMAPPort = defaults.imap_port
      form.value.UseTLS = defaults.imap_use_tls
      form.value.SMTPHost = defaults.smtp_host
      form.value.SMTPPort = defaults.smtp_port
    }
    form.value.IMAPUser = email
    form.value.SMTPUser = email
  }
}
</script>

<template>
  <div class="card">
    <header class="card__head">
      <h2>
        {{ props.editing ? t['settings.accounts.edit'] : t['settings.accounts.add'] }}
      </h2>
      <!-- Tab strip shares the header row with the title; the indicator rides on
           the header's own border-bottom (see .head-tabs below). -->
      <Tabs.Root v-if="!props.editing" v-model="tab" class="head-tabs">
        <Tabs.List class="head-tabs__list">
          <Tabs.Trigger class="head-tabs__trigger" value="manual">
            {{ t['settings.accounts.tab.manual'] }}
          </Tabs.Trigger>
          <Tabs.Trigger class="head-tabs__trigger" value="oauth">
            {{ t['settings.accounts.tab.oauth'] }}
          </Tabs.Trigger>
          <Tabs.Indicator class="head-tabs__indicator" />
        </Tabs.List>
      </Tabs.Root>
    </header>

    <div class="card__body">
      <div class="frow">
        <span class="flabel">{{ t['settings.accounts.avatar'] }}</span>
        <div class="avatar-area">
          <label class="avatar-box">
            <img
              v-if="props.avatarPreview"
              :src="props.avatarPreview"
              class="avatar-box__img"
              alt=""
            />
            <i v-else class="i-ri:add-line" aria-hidden="true" />
            <input
              type="file"
              accept="image/png,image/jpeg,image/gif,image/webp"
              class="avatar-box__input"
              @change="emit('avatarFile', $event)"
            />
          </label>
          <div class="avatar-hint">{{ t['settings.accounts.upload_avatar'] }}</div>
        </div>
      </div>

      <Field.Root v-if="tab === 'manual'" class="frow">
        <Field.Label class="flabel">{{ t['settings.accounts.name'] }}</Field.Label>
        <Field.Input
          class="input"
          :model-value="form.Name"
          :placeholder="t['settings.accounts.placeholder_name']"
          @update:model-value="form.Name = String($event)"
        />
      </Field.Root>

      <!-- 服务器配置 pane: the classic typed form -->
      <template v-if="tab === 'manual'">
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.email'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.Email"
            :placeholder="t['settings.accounts.placeholder_email']"
            @update:model-value="onEmailInput(String($event))"
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.provider'] }}</Field.Label>
          <UiSelect
            :options="props.providerOptions"
            :model-value="form.Provider"
            width="230px"
            @update:model-value="setProvider"
          />
        </Field.Root>

        <div class="sec">IMAP</div>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.imap_host'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.IMAPHost"
            :placeholder="t['settings.accounts.placeholder_imap_host']"
            @update:model-value="form.IMAPHost = String($event)"
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.imap_port'] }}</Field.Label>
          <UiNumberInput :value="form.IMAPPort" @value="form.IMAPPort = $event" />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.imap_user'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.IMAPUser"
            :placeholder="t['settings.accounts.placeholder_email']"
            @update:model-value="form.IMAPUser = String($event)"
          />
        </Field.Root>
        <!-- The password is not a password on several providers, and that note is
             about *this* field: as a Field.HelperText it sits under the control,
             reads as a hint instead of a page-wide banner, and Ark can describe
             the input with it. -->
        <Field.Root class="frow frow--stacked">
          <Field.Label class="flabel">{{ t['settings.accounts.imap_pass'] }}</Field.Label>
          <UiPasswordInput
            class="input"
            :value="form.IMAPPass"
            @input="form.IMAPPass = strVal($event)"
          />
          <Field.HelperText v-if="passwordHint" class="field__hint frow-hint field__hint--warn">
            {{ passwordHint }}
          </Field.HelperText>
        </Field.Root>

        <div class="sec">SMTP</div>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.smtp_host'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.SMTPHost"
            :placeholder="t['settings.accounts.placeholder_smtp_host']"
            @update:model-value="form.SMTPHost = String($event)"
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.smtp_port'] }}</Field.Label>
          <UiNumberInput :value="form.SMTPPort" @value="form.SMTPPort = $event" />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.smtp_user'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.SMTPUser"
            :placeholder="t['settings.accounts.placeholder_email']"
            @update:model-value="form.SMTPUser = String($event)"
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.smtp_pass'] }}</Field.Label>
          <UiPasswordInput
            class="input"
            :value="form.SMTPPass"
            @input="form.SMTPPass = strVal($event)"
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.use_tls'] }}</Field.Label>
          <UiSwitch :checked="form.UseTLS" @update:checked="form.UseTLS = $event" />
        </Field.Root>
      </template>

      <!-- 在线验证 pane: browser-based OAuth -->
      <template v-else>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.email'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.Email"
            :placeholder="t['settings.accounts.oauth.email_pending']"
            readonly
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.alias'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.Name"
            :placeholder="t['settings.accounts.placeholder_name']"
            @update:model-value="form.Name = String($event)"
          />
        </Field.Root>
        <Field.Root class="frow">
          <Field.Label class="flabel">{{ t['settings.accounts.oauth.provider'] }}</Field.Label>
          <UiSelect
            :options="oauthProviderOptions"
            :model-value="oauthProvider"
            width="230px"
            @update:model-value="oauthProvider = $event as OAuthProviderId"
          />
        </Field.Root>
        <!-- The Client ID is the one input in this pane you can get wrong, and
             both notes explain how to fill it (leave it empty, or bring your own
             Google Cloud app). As helper text they stay where they were but stop
             reading like app-level alerts. -->
        <Field.Root class="frow frow--stacked">
          <Field.Label class="flabel">
            {{ t['settings.accounts.oauth.client_id'] }}
          </Field.Label>
          <Field.Input
            class="input"
            :model-value="oauthClientID"
            :placeholder="t['settings.accounts.oauth.client_id_placeholder']"
            @update:model-value="oauthClientID = String($event)"
          />
          <Field.HelperText class="frow-hint">
            {{ t['settings.accounts.oauth.hint'] }}
          </Field.HelperText>
          <Field.HelperText v-if="oauthProvider === 'google'" class="frow-hint field__hint--warn">
            {{ t['settings.accounts.oauth.google_note'] }}
          </Field.HelperText>
        </Field.Root>

        <div class="frow">
          <span class="flabel" aria-hidden="true"></span>
          <BusyButton
            class="btn"
            :label="t['settings.accounts.oauth.begin']"
            :busy="oauthWaiting"
            :disabled="oauthWaiting"
            @click="startOAuth"
          />
        </div>

        <div v-if="oauthWaiting" class="hint hint--ok">
          {{ t['settings.accounts.oauth.waiting'] }}
        </div>
        <div v-else-if="oauthResult" class="hint hint--ok">
          {{ t['settings.accounts.oauth.success'].replace('{email}', oauthResult.email ?? '') }}
        </div>
        <div v-if="oauthError" class="hint hint--err">{{ oauthError }}</div>
        <div v-else-if="isOAuth && !oauthResult" class="hint">
          {{ t['settings.accounts.oauth.is_oauth'] }}
        </div>
      </template>

      <div class="sec">{{ t['settings.accounts.sync'] }}</div>
      <!-- Two columns instead of the usual flex .frow: the helper line belongs to
           the field (Ark renders it as Field.HelperText and can describe the
           control with it) but has to sit on its own line under the control, not
           beside the switch. -->
      <Field.Root class="frow frow--stacked">
        <Field.Label class="flabel">{{ t['settings.accounts.auto_sync'] }}</Field.Label>
        <div class="sync-row">
          <UiSwitch :checked="form.SyncInterval > 0" @update:checked="onAutoSyncToggle" />
          <template v-if="form.SyncInterval > 0">
            <UiNumberInput :value="form.SyncInterval" @value="form.SyncInterval = $event" />
            <span class="muted">{{ t['settings.accounts.sync_minutes'] }}</span>
          </template>
          <span v-else class="muted">{{ t['settings.accounts.sync_disabled'] }}</span>
        </div>
        <Field.HelperText v-if="form.SyncInterval > 0" class="frow-hint">
          {{ t['settings.accounts.sync_hint'] }}
        </Field.HelperText>
      </Field.Root>

      <div
        v-if="props.testResult"
        class="hint"
        :class="props.testResult.success ? 'hint--ok' : 'hint--err'"
      >
        {{ props.testResult.message }}
      </div>
    </div>

    <footer class="card__foot">
      <button type="button" class="btn" @click="emit('cancel')">
        {{ t['settings.accounts.cancel'] }}
      </button>
      <BusyButton
        class="btn"
        :label="t['settings.accounts.test']"
        :busy="props.testing"
        @click="emit('test')"
      />
      <BusyButton
        class="btn btn--primary"
        :label="t['settings.accounts.save']"
        :busy="props.saving"
        @click="emit('save')"
      />
    </footer>
  </div>
</template>

<style scoped>
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
/* Tab strip in the header: stretch over the head's vertical padding so the
   selected-tab indicator lands exactly on the header's border-bottom, like the
   reminders page's tab strip rides on its own bottom border. display:flex is
   what makes the list (align-self: stretch) actually fill the stretched box —
   without it the list kept its text height and sat at the top, pushing the
   labels far above center. */
.head-tabs {
  display: flex;
  align-self: stretch;
  margin: -10px 0;
}
.head-tabs__list {
  position: relative;
  display: flex;
  align-self: stretch;
  align-items: stretch;
  gap: 2px;
  padding: 0 4px;
}
.head-tabs__trigger {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  padding: 0 12px;
  border: none;
  background: transparent;
  color: var(--color-text-3);
  font-size: 13px;
  cursor: pointer;
}
.head-tabs__trigger:hover {
  color: var(--n-text-color);
}
.head-tabs__trigger[data-selected] {
  color: var(--n-primary-color);
  font-weight: 600;
}
.head-tabs__indicator {
  position: absolute;
  bottom: -1px;
  height: 2px;
  background: var(--n-primary-color);
  transition:
    left 0.2s ease,
    width 0.2s ease;
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
/* Label column + control column, with the helper line as a second row under the
   control only. Grid rather than flex so the helper text can break onto its own
   line without a wrapper element and without a hand-tuned padding-left. */
.frow--stacked {
  display: grid;
  grid-template-columns: 108px 1fr;
  column-gap: 12px;
  row-gap: 0;
}
.flabel {
  flex: 0 0 108px;
}
.input {
  flex: 1;
  min-width: 0;
}
.sec {
  padding: 12px 0 4px;
  color: var(--color-text-2);
  font-size: 13px;
  font-weight: 600;
}
/* `.hint` is now only for run-time status (test result, OAuth progress): a
   bordered strip is right for "something just happened", wrong for a standing
   note about a field — those are Field.HelperText now. */
.hint {
  margin: 10px 0;
  padding: 8px 12px;
  color: var(--n-text-color);
  font-size: 13px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
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
/* The visible number box is UiNumberInput's `.num` control — an element *inside*
   the child component. A scoped class on the tag only reaches the child's root
   element (NumberInput.Root's wrapper div), so the old `.input--sm` never applied:
   the box fell back to `.num { width: 100% }` and sized itself against a
   shrink-to-fit row, which is what made the row look off-centre. */
.sync-row :deep(.num) {
  flex: 0 0 88px;
}
/* Field.HelperText renders a <span>; the grid item makes it a block on its own row. */
.frow-hint {
  grid-column: 2;
  margin: 4px 0 0;
  color: var(--color-text-3);
  font-size: 12px;
  line-height: 1.5;
}
/* Kept in this file, after .frow-hint: scoped styles carry an extra attribute
   selector, so they outrank the global .field__hint--warn in ui.css. */
.field__hint--warn {
  color: var(--color-warning);
}
.avatar-area {
  display: flex;
  align-items: center;
  gap: 12px;
}
.avatar-box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  overflow: hidden;
  color: var(--color-text-3);
  cursor: pointer;
  background: var(--n-action-color);
  border: 1px dashed var(--n-border-color);
  border-radius: 10px;
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
@media (max-width: 560px) {
  .frow {
    align-items: flex-start;
    flex-direction: column;
    gap: 5px;
  }
  .flabel {
    flex-basis: auto;
  }
  .input {
    width: 100%;
    flex-basis: auto;
  }
  /* Column layout: one column, so the helper text lines up with everything else. */
  .frow--stacked {
    grid-template-columns: 1fr;
  }
  .frow-hint {
    grid-column: 1;
  }
  /* Column layout: the number box takes the row's full width like every other
     input. */
  .sync-row :deep(.num) {
    flex: 1 1 auto;
  }
}
</style>
