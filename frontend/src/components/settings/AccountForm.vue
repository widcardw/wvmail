<script setup lang="ts">
import { useI18n } from '~/i18n'
import { Field } from '@ark-ui/vue/field'
import BusyButton from '~/components/ui/BusyButton.vue'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiPasswordInput from '~/components/ui/UiPasswordInput.vue'
import UiNumberInput from '~/components/ui/UiNumberInput.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
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

      <Field.Root class="frow">
        <Field.Label class="flabel">{{ t['settings.accounts.name'] }}</Field.Label>
        <Field.Input
          class="input"
          :model-value="form.Name"
          :placeholder="t['settings.accounts.placeholder_name']"
          @update:model-value="form.Name = String($event)"
        />
      </Field.Root>
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
      <Field.Root class="frow">
        <Field.Label class="flabel">{{ t['settings.accounts.imap_pass'] }}</Field.Label>
        <UiPasswordInput
          class="input"
          :value="form.IMAPPass"
          @input="form.IMAPPass = strVal($event)"
        />
      </Field.Root>

      <div v-if="props.is163Provider" class="hint hint--warn">
        163 {{ t['settings.accounts.imap_warning'] }}
      </div>
      <div v-else-if="props.needsAuthCode" class="hint hint--warn">
        {{ t['settings.accounts.auth_code_hint'] }}
      </div>

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

      <div class="sec">{{ t['settings.accounts.sync'] }}</div>
      <Field.Root class="frow">
        <Field.Label class="flabel">{{ t['settings.accounts.auto_sync'] }}</Field.Label>
        <div class="sync-row">
          <UiSwitch :checked="form.SyncInterval > 0" @update:checked="onAutoSyncToggle" />
          <template v-if="form.SyncInterval > 0">
            <UiNumberInput
              class="input--sm"
              :value="form.SyncInterval"
              @value="form.SyncInterval = $event"
            />
            <span class="muted">{{ t['settings.accounts.sync_minutes'] }}</span>
          </template>
          <span v-else class="muted">{{ t['settings.accounts.sync_disabled'] }}</span>
        </div>
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
  .input--sm {
    width: 100%;
    flex-basis: auto;
  }
}
</style>
