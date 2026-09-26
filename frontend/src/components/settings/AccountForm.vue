<script setup lang="ts">
import { useI18n } from '~/i18n'
import BusyButton from '~/components/BusyButton.vue'
import UiSelect from '~/components/UiSelect.vue'
import UiSwitch from '~/components/UiSwitch.vue'
import { EMAIL_PROVIDER_DEFAULTS, detectProvider } from '~/types/email'
import type { EmailAccount, EmailProvider } from '~/types/email'

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

function strVal(e: Event): string {
  return (e.target as HTMLInputElement).value
}

function intVal(e: Event): number {
  return Number((e.target as HTMLInputElement).value) || 0
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

      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.name'] }}</label>
        <input
          class="input"
          :value="form.Name"
          @input="form.Name = strVal($event)"
          :placeholder="t['settings.accounts.placeholder_name']"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.email'] }}</label>
        <input
          class="input"
          :value="form.Email"
          @input="onEmailInput(strVal($event))"
          :placeholder="t['settings.accounts.placeholder_email']"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.provider'] }}</label>
        <UiSelect
          :options="props.providerOptions"
          :model-value="form.Provider"
          width="230px"
          @update:model-value="setProvider"
        />
      </div>

      <div class="sec">IMAP</div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.imap_host'] }}</label>
        <input
          class="input"
          :value="form.IMAPHost"
          @input="form.IMAPHost = strVal($event)"
          :placeholder="t['settings.accounts.placeholder_imap_host']"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.imap_port'] }}</label>
        <input
          class="input input--num"
          type="number"
          :value="form.IMAPPort"
          @change="form.IMAPPort = intVal($event)"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.imap_user'] }}</label>
        <input
          class="input"
          :value="form.IMAPUser"
          @input="form.IMAPUser = strVal($event)"
          :placeholder="t['settings.accounts.placeholder_email']"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.imap_pass'] }}</label>
        <input
          class="input"
          type="password"
          :value="form.IMAPPass"
          @input="form.IMAPPass = strVal($event)"
        />
      </div>

      <div v-if="props.is163Provider" class="hint hint--warn">
        163 {{ t['settings.accounts.imap_warning'] }}
      </div>
      <div v-else-if="props.needsAuthCode" class="hint hint--warn">
        {{ t['settings.accounts.auth_code_hint'] }}
      </div>

      <div class="sec">SMTP</div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.smtp_host'] }}</label>
        <input
          class="input"
          :value="form.SMTPHost"
          @input="form.SMTPHost = strVal($event)"
          :placeholder="t['settings.accounts.placeholder_smtp_host']"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.smtp_port'] }}</label>
        <input
          class="input input--num"
          type="number"
          :value="form.SMTPPort"
          @change="form.SMTPPort = intVal($event)"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.smtp_user'] }}</label>
        <input
          class="input"
          :value="form.SMTPUser"
          @input="form.SMTPUser = strVal($event)"
          :placeholder="t['settings.accounts.placeholder_email']"
        />
      </div>
      <div class="frow">
        <label class="flabel">{{ t['settings.accounts.smtp_pass'] }}</label>
        <input
          class="input"
          type="password"
          :value="form.SMTPPass"
          @input="form.SMTPPass = strVal($event)"
        />
      </div>
      <div class="frow">
        <span class="flabel">{{ t['settings.accounts.use_tls'] }}</span>
        <UiSwitch :checked="form.UseTLS" @update:checked="form.UseTLS = $event" />
      </div>

      <div class="sec">{{ t['settings.accounts.sync'] }}</div>
      <div class="frow">
        <span class="flabel">{{ t['settings.accounts.auto_sync'] }}</span>
        <div class="sync-row">
          <UiSwitch
            :checked="form.SyncInterval > 0"
            @update:checked="onAutoSyncToggle"
          />
          <template v-if="form.SyncInterval > 0">
            <input
              class="input input--sm"
              type="number"
              :value="form.SyncInterval"
              @change="form.SyncInterval = intVal($event)"
            />
            <span class="muted">{{ t['settings.accounts.sync_minutes'] }}</span>
          </template>
          <span v-else class="muted">{{ t['settings.accounts.sync_disabled'] }}</span>
        </div>
      </div>

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
.flabel {
  flex: 0 0 108px;
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
  padding: 12px 0 4px;
  color: var(--color-text-2);
  font-size: 13px;
  font-weight: 600;
}
.hint {
  margin: 10px 0;
  padding: 8px 12px;
  color: var(--n-text-color);
  font-size: 13px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
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
  .input,
  .input--num,
  .input--sm {
    width: 100%;
    flex-basis: auto;
  }
}
</style>
