<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { Field } from '@ark-ui/vue/field'
import { SegmentGroup } from '@ark-ui/vue/segment-group'
import BusyButton from '~/components/ui/BusyButton.vue'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiPasswordInput from '~/components/ui/UiPasswordInput.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import { proxyModeLabels } from '~/composables/proxy'
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

const { t, tf } = useI18n()

// Two ways to add an account: type the server details by hand, or finish a
// browser-based OAuth round trip. Adding defaults to the manual form; editing an
// OAuth account opens on the online-verification pane so a re-auth is one click
// away (the switcher is hidden while editing and the pane follows the account's
// stored auth type).
//
// The switcher is a SegmentGroup, not Ark Tabs. `Tabs.Content` reads the tab
// machine out of context, so it only renders *inside* `Tabs.Root` — and here the
// strip sits in the card header while the panes sit in the card body, with the
// avatar row and the sync section shared by both. Making Tabs work would mean
// wrapping the entire card in `Tabs.Root`, i.e. handing the card's own markup to
// the tab machine for the sake of a two-item switch. A radio group owns only the
// value, which is all this needs: the panes stay plain `v-if` branches.
const tab = ref<string>(props.editing && form.value.AuthType === 'oauth2' ? 'oauth' : 'manual')

/**
 * SegmentGroup can report `null` (its `clearValue`), but "server settings or
 * online sign-in" has no empty state — keeping the last value means the two panes
 * can never both be hidden.
 */
function onTabChange(value: string | null) {
  if (value) tab.value = value
}

const addModes = computed(() => [
  { value: 'manual', label: t.value['settings.accounts.tab.manual'] },
  { value: 'oauth', label: t.value['settings.accounts.tab.oauth'] },
])

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

type StatusKind = 'info' | 'ok' | 'err'

const STATUS_ICON: Record<StatusKind, string> = {
  info: 'i-ri:information-line',
  ok: 'i-ri:checkbox-circle-line',
  err: 'i-ri:error-warning-line',
}

/**
 * The single line under the "authorise" button. OAuth is a sequence of run-time
 * events, so it reads as one status line whose colour carries the outcome —
 * previously three independent banners could stack, and a boxed strip for one
 * sentence shouted louder than the form it sat in.
 */
const oauthStatus = computed<{ kind: StatusKind; text: string } | null>(() => {
  if (oauthError.value) return { kind: 'err', text: oauthError.value }
  if (oauthWaiting.value) return { kind: 'info', text: t.value['settings.accounts.oauth.waiting'] }
  if (oauthResult.value) {
    return {
      kind: 'ok',
      text: tf('settings.accounts.oauth.success', { email: oauthResult.value.email ?? '' }),
    }
  }
  return null
})

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

// --- proxy ---
//
// The stored shape is { Mode, URL }: Mode "" follows the global proxy, "custom"
// uses URL, "off" never proxies. `Proxy` is always an object on the form (see
// emptyForm in Account.vue), so these helpers never have to null-check the
// parent, only the URL.
const proxyMode = computed(() => form.value.Proxy?.Mode ?? '')

const proxyModeOptions = computed(() => proxyModeLabels(t.value))

function setProxyMode(mode: string) {
  form.value.Proxy = { Mode: mode, URL: mode === 'custom' ? (form.value.Proxy?.URL ?? '') : '' }
}

function setProxyURL(url: string) {
  form.value.Proxy = { Mode: 'custom', URL: url }
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
  <div class="surface">
    <header class="surface__head">
      <h2>
        {{ props.editing ? t['settings.accounts.edit'] : t['settings.accounts.add'] }}
      </h2>
      <!-- The add-mode switch. It rides on the header's own border-bottom, so the
           indicator has to be an absolutely positioned bar measured off the
           selected item — that is what SegmentGroup.Indicator hands us through
           --left / --width. `orientation` is not optional: zag only writes
           `left: var(--left)` when the orientation is literally "horizontal"
           (it has no default), and without it the bar is pinned to `top` and
           never follows the selected item. The label is for screen readers only:
           the visible heading names the card, not the choice. -->
      <SegmentGroup.Root
        v-if="!props.editing"
        class="head-tabs"
        orientation="horizontal"
        :model-value="tab"
        @update:model-value="onTabChange"
      >
        <SegmentGroup.Label class="vh">{{ t['settings.accounts.tab.group'] }}</SegmentGroup.Label>
        <SegmentGroup.Indicator class="head-tabs__indicator" />
        <SegmentGroup.Item
          v-for="mode in addModes"
          :key="mode.value"
          class="head-tabs__item"
          :value="mode.value"
        >
          <SegmentGroup.ItemText>{{ mode.label }}</SegmentGroup.ItemText>
          <SegmentGroup.ItemHiddenInput />
        </SegmentGroup.Item>
      </SegmentGroup.Root>
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
          <Field.Label class="flabel">{{ t['settings.accounts.alias'] }}</Field.Label>
          <Field.Input
            class="input"
            :model-value="form.Name"
            :placeholder="t['settings.accounts.placeholder_name']"
            @update:model-value="form.Name = String($event)"
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
        <!-- Server and port share one line. They stay two separate Ark fields:
             a Field.Root owns a single control id, so one root for both would
             give the two controls the same id and point the label at both. -->
        <div class="frow frow--pair">
          <Field.Root class="frow__cell frow__cell--grow">
            <Field.Label class="flabel">{{ t['settings.accounts.imap_host'] }}</Field.Label>
            <Field.Input
              class="input"
              :model-value="form.IMAPHost"
              :placeholder="t['settings.accounts.placeholder_imap_host']"
              @update:model-value="form.IMAPHost = String($event)"
            />
          </Field.Root>
          <Field.Root class="frow__cell frow__cell--port">
            <Field.Label class="flabel--inline">{{ t['settings.accounts.port'] }}</Field.Label>
            <UiNumberInput :value="form.IMAPPort" width="96px" @value="form.IMAPPort = $event" />
          </Field.Root>
        </div>
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
          <UiPasswordInput :value="form.IMAPPass" @input="form.IMAPPass = strVal($event)" />
          <Field.HelperText v-if="passwordHint" class="field__hint frow-hint field__hint--warn">
            {{ passwordHint }}
          </Field.HelperText>
        </Field.Root>

        <div class="sec">SMTP</div>
        <div class="frow frow--pair">
          <Field.Root class="frow__cell frow__cell--grow">
            <Field.Label class="flabel">{{ t['settings.accounts.smtp_host'] }}</Field.Label>
            <Field.Input
              class="input"
              :model-value="form.SMTPHost"
              :placeholder="t['settings.accounts.placeholder_smtp_host']"
              @update:model-value="form.SMTPHost = String($event)"
            />
          </Field.Root>
          <Field.Root class="frow__cell frow__cell--port">
            <Field.Label class="flabel--inline">{{ t['settings.accounts.port'] }}</Field.Label>
            <UiNumberInput :value="form.SMTPPort" width="96px" @value="form.SMTPPort = $event" />
          </Field.Root>
        </div>
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
          <UiPasswordInput :value="form.SMTPPass" @input="form.SMTPPass = strVal($event)" />
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
            width="400px"
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

        <!-- One status slot, not three stacked banners: waiting → success/failure
             is a single run-time state, and the live region announces the change
             because the element stays mounted between states. -->
        <div
          v-if="oauthStatus"
          class="status"
          :class="`status--${oauthStatus.kind}`"
          :role="oauthStatus.kind === 'err' ? 'alert' : 'status'"
        >
          <i :class="STATUS_ICON[oauthStatus.kind]" aria-hidden="true" />
          <span>{{ oauthStatus.text }}</span>
        </div>
        <!-- A standing fact about the stored auth type, not an event — muted text
             rather than a status line, and it yields to any live status above. -->
        <p v-else-if="isOAuth" class="pane-note">
          {{ t['settings.accounts.oauth.is_oauth'] }}
        </p>
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

      <!-- Proxy: three answers, not two. "No proxy" exists because a mailbox on
           the local network must be able to opt out of a global proxy, and
           "follow" must stay distinguishable from it. -->
      <div class="sec">{{ t['settings.accounts.proxy'] }}</div>
      <Field.Root class="frow">
        <Field.Label class="flabel">{{ t['settings.accounts.proxy_mode'] }}</Field.Label>
        <UiSelect
          :options="proxyModeOptions"
          :model-value="proxyMode"
          width="230px"
          @update:model-value="setProxyMode"
        />
      </Field.Root>
      <Field.Root v-if="proxyMode === 'custom'" class="frow frow--stacked">
        <Field.Label class="flabel">{{ t['settings.accounts.proxy_address'] }}</Field.Label>
        <Field.Input
          class="input"
          :model-value="form.Proxy?.URL ?? ''"
          :placeholder="t['settings.proxy.address_placeholder']"
          @update:model-value="setProxyURL(String($event))"
        />
        <Field.HelperText class="frow-hint">
          {{ t['settings.accounts.proxy_hint'] }}
        </Field.HelperText>
      </Field.Root>

      <!-- Same status line as the OAuth pane: "the test just ran" is an event, but
           it does not deserve a bordered box around one sentence. -->
      <div
        v-if="props.testResult"
        class="status"
        :class="props.testResult.success ? 'status--ok' : 'status--err'"
        role="status"
      >
        <i
          :class="props.testResult.success ? STATUS_ICON.ok : STATUS_ICON.err"
          aria-hidden="true"
        />
        <span>{{ props.testResult.message }}</span>
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
/* The add-mode switch: display:flex + align-self:stretch make the strip fill the
   header's padding box (the -10px margins cancel the header's vertical padding),
   so the indicator's -1px offset lands exactly on the header's border-bottom.
   `position: relative` comes from Ark itself (SegmentGroup.Root sets it inline),
   which is also what makes the indicator's --left/--width meaningful. */
.head-tabs {
  display: flex;
  align-self: stretch;
  align-items: stretch;
  gap: 2px;
  padding: 0 4px;
  margin: -10px 0;
}
.head-tabs__item {
  position: relative; /* contains ItemHiddenInput's absolutely positioned radio */
  display: flex;
  align-items: center;
  padding: 0 12px;
  color: var(--color-text-3);
  font-size: 13px;
  cursor: pointer;
  user-select: none;
}
.head-tabs__item:hover {
  color: var(--n-text-color);
}
.head-tabs__item[data-state='checked'] {
  color: var(--n-primary-color);
  font-weight: 600;
}
/* Focus lands on the hidden radio, so the ring has to be drawn on the item. */
.head-tabs__item[data-focus-visible] {
  outline: 2px solid var(--n-primary-color);
  outline-offset: -2px;
  border-radius: 4px;
}
/* Ark supplies position/left/--left/--width and the slide transition inline; only
   the underline's thickness and the axis it hangs from belong here. Without
   `width: var(--width)` the bar is shrink-to-fit and renders zero-width. */
.head-tabs__indicator {
  bottom: -1px;
  width: var(--width);
  height: 2px;
  background: var(--n-primary-color);
}
/* Screen-reader-only text: the group needs a name, but the visible heading names
   the card. Absolutely positioned inside .head-tabs, so it cannot push layout. */
.vh {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  white-space: nowrap;
  clip-path: inset(50%);
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
/* Two related controls on one line (server + port). The port keeps its own
   small inline label so it stays identifiable once it is no longer the only
   thing in the row. */
.frow--pair {
  gap: 12px;
}
.frow__cell {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.frow__cell--grow {
  flex: 1 1 auto;
}
.frow__cell--port {
  flex: 0 0 auto;
  gap: 8px;
}
/* The inline label sits before its control instead of on the row's label
   column, so it must not claim that column's width. */
.flabel--inline {
  flex: 0 0 auto;
  font-size: 12px;
  color: var(--color-text-3);
  line-height: 1;
  white-space: nowrap;
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
/* Shared status styles live in ui.css; only the spacing sits here because this
   is inside the form body (not a section-level hint). */
.status {
  margin: 8px 0;
}
/* A standing note about the account (not an event): muted, no icon. */
.pane-note {
  margin: 8px 0;
  color: var(--color-text-3);
  font-size: 12px;
  line-height: 1.5;
}
.sync-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
/* UiNumberInput renders `.num` as the component root, so this scoped descendant
   selector can size the visible box without a wrapper around it. */
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
  /* Column layout: the port box drops under the server box and spans the row,
     like every other input. */
  .frow__cell {
    flex-direction: column;
    align-items: stretch;
    width: 100%;
  }
  .frow__cell--port {
    flex: 0 0 auto;
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
