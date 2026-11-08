<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { usePreferencesStore } from '~/stores/preferences'
import { useFeedbackStore } from '~/stores/feedback'
import { proxyModeLabels, proxyModeOf, resolveProxyRoute } from '~/composables/proxy'
import type { ProxyMode } from '~/composables/proxy'
import { NetworkService } from '#/wvmail/services'
import type { EmailAccount } from '~/types/email'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import UiSelect from '~/components/ui/UiSelect.vue'
import UiFieldRow from '~/components/ui/UiFieldRow.vue'
import BusyButton from '~/components/ui/BusyButton.vue'
import AccountAvatar from '~/components/icon/AccountAvatar.vue'

const { t } = useI18n()
const preferences = usePreferencesStore()
const accounts = useAccountsStore()
const feedback = useFeedbackStore()

/**
 * The address box is a draft, not the stored value: it is written on blur (or
 * Enter), so a half-typed "127.0.0.1:78" never reaches config.yaml and never
 * fails validation while the user is still typing.
 */
const draftUrl = ref(preferences.proxyUrl)
const saving = ref(false)

onMounted(async () => {
  draftUrl.value = preferences.proxyUrl
  if (accounts.accounts.length === 0) await accounts.loadAccounts()
})

function currentGlobal() {
  return { enabled: preferences.proxyEnabled, url: preferences.proxyUrl }
}

async function saveGlobal(enabled: boolean, url: string) {
  if (saving.value) return
  saving.value = true
  try {
    await preferences.setProxy(enabled, url)
  } catch (e) {
    // Go validates the address on save; showing its message is more useful than
    // a generic "failed", and the store has already rolled the state back.
    feedback.error(t('settings.proxy.save_failed', { message: String(e) }))
    draftUrl.value = preferences.proxyUrl
  } finally {
    saving.value = false
  }
}

function onToggle(checked: boolean) {
  void saveGlobal(checked, draftUrl.value.trim())
}

function onCommitUrl() {
  const next = draftUrl.value.trim()
  if (next === preferences.proxyUrl) return
  void saveGlobal(preferences.proxyEnabled, next)
}

function onUrlKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter') (event.target as HTMLInputElement).blur()
}

// --- test button ---

type TestState = { ok: boolean; text: string } | null
const testing = ref(false)
const testResult = ref<TestState>(null)

async function runTest() {
  const address = draftUrl.value.trim()
  if (!address || testing.value) return
  testing.value = true
  testResult.value = null
  try {
    const res = await NetworkService.TestProxy(address)
    testResult.value = res.Ok
      ? { ok: true, text: t('settings.proxy.test_ok', { ms: res.LatencyMS }) }
      : { ok: false, text: `${t('settings.proxy.test_fail')} · ${res.Error}` }
  } catch (e) {
    testResult.value = { ok: false, text: `${t('settings.proxy.test_fail')} · ${String(e)}` }
  } finally {
    testing.value = false
  }
}

// --- per-account modes ---

const modeOptions = computed(() => proxyModeLabels(t))

function modeOf(account: EmailAccount): string {
  return proxyModeOf(account.Proxy)
}

/** The one line under each account: where its mail actually goes. */
function routeText(account: EmailAccount): string {
  const route = resolveProxyRoute(account.Proxy, currentGlobal())
  switch (route.kind) {
    case 'custom':
      return route.address
        ? t('settings.proxy.route_custom', { address: route.address })
        : t('settings.proxy.route_custom_missing')
    case 'global':
      return t('settings.proxy.route_global', { address: route.address })
    default:
      return t('settings.proxy.route_direct')
  }
}

/**
 * Switch one account between the three modes.
 *
 * Switching to "custom" seeds the address with the global one, because a mode
 * with an empty address is a setting that cannot work — the dial fails with "no
 * address" and the user has no clue where to fix it. With nothing to seed (no
 * global proxy configured either) the change is refused with a message instead of
 * written: this control picks a route, it does not type an address, and the
 * account form is where one gets typed.
 */
async function setMode(account: EmailAccount, mode: string) {
  const nextMode = mode as ProxyMode
  if (nextMode === proxyModeOf(account.Proxy)) return

  const seeded = nextMode === 'custom' ? (account.Proxy?.URL || preferences.proxyUrl).trim() : ''
  if (nextMode === 'custom' && !seeded) {
    feedback.error(t('settings.proxy.account_needs_address'))
    return
  }

  try {
    await accounts.setAccountProxy(account.ID, { Mode: nextMode, URL: seeded })
  } catch (e) {
    feedback.error(t('settings.proxy.account_failed', { message: String(e) }))
  }
}
</script>

<template>
  <div class="wrap">
    <!-- The global proxy. One switch and one address: everything outbound —
         IMAP, the LLM calls, translation, OAuth — follows it. -->
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t('settings.proxy.title') }}</h2>
      </header>

      <UiFieldRow :label="t('settings.proxy.global')">
        <UiSwitch :checked="preferences.proxyEnabled" @update:checked="onToggle" />
      </UiFieldRow>

      <div class="row">
        <span class="row__label">{{ t('settings.proxy.address') }}</span>
        <input
          v-model="draftUrl"
          class="input addr"
          type="text"
          spellcheck="false"
          autocomplete="off"
          :placeholder="t('settings.proxy.address_placeholder')"
          :aria-label="t('settings.proxy.address')"
          @blur="onCommitUrl"
          @keydown="onUrlKeydown"
        />
        <BusyButton
          class="btn"
          :label="t('settings.proxy.test')"
          :busy="testing"
          :disabled="!draftUrl.trim()"
          @click="runTest"
        />
      </div>

      <div
        v-if="testResult"
        class="status"
        :class="testResult.ok ? 'status--ok' : 'status--err'"
        role="status"
      >
        <i
          :class="testResult.ok ? 'i-ri:checkbox-circle-line' : 'i-ri:error-warning-line'"
          aria-hidden="true"
        />
        <span>{{ testResult.text }}</span>
      </div>

      <p class="hint">{{ t('settings.proxy.hint') }}</p>
    </section>

    <!-- Per-account overrides. A select rather than a switch: "off" (never
         proxy, even globally) and "follow" are different answers, and a switch
         can only carry two. -->
    <section class="surface surface--spaced">
      <header class="surface__head">
        <h2>{{ t('settings.proxy.accounts') }}</h2>
      </header>

      <p v-if="accounts.accounts.length === 0" class="hint">
        {{ t('settings.proxy.no_accounts') }}
      </p>
      <ul v-else class="acc-list">
        <li v-for="account in accounts.accounts" :key="account.ID" class="acc">
          <AccountAvatar :account-id="account.ID" :size="26" />
          <div class="acc__text">
            <div class="acc__name">{{ account.Name || account.Email }}</div>
            <div class="acc__route">{{ routeText(account) }}</div>
          </div>
          <UiSelect
            :options="modeOptions"
            :model-value="modeOf(account)"
            width="150px"
            @update:model-value="setMode(account, $event)"
          />
        </li>
      </ul>

      <p class="hint">{{ t('settings.proxy.accounts_hint') }}</p>
    </section>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 18px;
}
/* Ark Switch: the root is a <label> for the hidden input, so the whole row is
   clickable. Part visuals live in styles/ui.css. */
.switch-row {
  cursor: pointer;
}
.addr {
  flex: 1;
  min-width: 0;
  max-width: 320px;
}
.acc-list {
  margin: 0;
  padding: 0;
  list-style: none;
}
.acc {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 18px;
  border-top: 1px solid var(--n-border-color);
}
.acc:first-child {
  border-top: none;
}
.acc__text {
  flex: 1;
  min-width: 0;
}
.acc__name {
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.acc__route {
  margin-top: 2px;
  color: var(--color-text-3);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
