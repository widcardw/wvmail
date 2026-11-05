<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { usePreferencesStore } from '~/stores/preferences'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import UiFieldRow from '~/components/ui/UiFieldRow.vue'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
import UiIconAction from '~/components/ui/UiIconAction.vue'

const { t } = useI18n()
const preferences = usePreferencesStore()

const newSender = ref('')
const senderError = ref<string | null>(null)
const editingSender = ref<string | null>(null)
const editSenderValue = ref('')
const editSenderError = ref<string | null>(null)
const imageProxyURL = ref(preferences.imageProxyURL)
const imageProxyError = ref<string | null>(null)

watch(
  () => preferences.imageProxyURL,
  (value) => {
    imageProxyURL.value = value
  },
)

function onAllowRemoteChange(checked: boolean) {
  void preferences.setAllowRemoteContent(checked)
}

function onBlockSuspiciousChange(checked: boolean) {
  void preferences.setBlockSuspiciousMail(checked)
}

async function saveImageProxyURL() {
  const value = imageProxyURL.value.trim()
  imageProxyError.value = null
  if (value && !/^https?:\/\/[^/\s]+/i.test(value)) {
    imageProxyError.value = t.value['settings.remote.proxy_invalid']
    return
  }
  try {
    await preferences.setImageProxyURL(value)
    imageProxyURL.value = value
  } catch {
    imageProxyError.value = t.value['settings.remote.proxy_invalid']
  }
}

function onRemoveSender(sender: string) {
  void preferences.removeAllowedRemoteSender(sender)
}

async function addSender() {
  const value = newSender.value.trim()
  senderError.value = null
  const result = await preferences.addRemoteSenderPattern(value)
  if (result === 'invalid') {
    senderError.value = t.value['settings.remote.add_invalid']
    return
  }
  if (result === 'duplicate') {
    senderError.value = t.value['settings.remote.duplicate']
    return
  }
  newSender.value = ''
}

function beginEditSender(sender: string) {
  editingSender.value = sender
  editSenderValue.value = sender
  editSenderError.value = null
  senderError.value = null
}

function cancelEditSender() {
  editingSender.value = null
  editSenderValue.value = ''
  editSenderError.value = null
}

async function saveSender(sender: string) {
  editSenderError.value = null
  const result = await preferences.updateAllowedRemoteSender(sender, editSenderValue.value)
  if (result === 'invalid') {
    editSenderError.value = t.value['settings.remote.add_invalid']
    return
  }
  if (result === 'duplicate') {
    editSenderError.value = t.value['settings.remote.duplicate']
    return
  }
  cancelEditSender()
}
</script>

<template>
  <div class="wrap">
    <!-- Remote content: the global switch plus the per-sender allow list it feeds.
         Both halves belong together — the list is the switch's exception, and the
         reader's "always allow this sender" writes into it (see
         composables/html/remote.ts). -->
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['settings.remote.title'] }}</h2>
      </header>

      <UiFieldRow :label="t['settings.remote.allow']">
        <UiSwitch :checked="preferences.allowRemoteContent" @update:checked="onAllowRemoteChange" />
      </UiFieldRow>

      <UiFieldRow :label="t['settings.remote.proxy']" :hint="t['settings.remote.proxy_hint']">
        <input
          v-model="imageProxyURL"
          class="input proxy__input"
          type="url"
          spellcheck="false"
          autocomplete="off"
          :placeholder="t['settings.remote.proxy_placeholder']"
          :aria-invalid="Boolean(imageProxyError)"
          @input="imageProxyError = null"
          @change="saveImageProxyURL"
          @keydown.enter.prevent="saveImageProxyURL"
        />
        <p v-if="imageProxyError" class="proxy__error" role="alert">{{ imageProxyError }}</p>
      </UiFieldRow>

      <div class="row row--top">
        <span class="row__label">{{ t['settings.remote.hint'] }}</span>
        <div class="senders">
          <form class="senders__add" @submit.prevent="addSender">
            <input
              v-model="newSender"
              class="input senders__input"
              type="text"
              spellcheck="false"
              autocomplete="off"
              :placeholder="t['settings.remote.add_placeholder']"
              :aria-invalid="Boolean(senderError)"
              @input="senderError = null"
            />
            <button type="submit" class="btn btn--sm btn--primary" :disabled="!newSender.trim()">
              {{ t['settings.remote.add'] }}
            </button>
          </form>
          <p v-if="senderError" class="senders__error" role="alert">
            {{ senderError }}
          </p>
          <p v-if="preferences.allowedRemoteSenders.length === 0" class="senders__empty">
            {{ t['settings.remote.empty'] }}
          </p>
          <UiScrollArea v-else class="senders__scroll">
            <ul class="senders__list">
              <li
                v-for="sender in preferences.allowedRemoteSenders"
                :key="sender"
                class="senders__item"
              >
                <form
                  v-if="editingSender === sender"
                  class="senders__edit"
                  @submit.prevent="saveSender(sender)"
                >
                  <div class="senders__edit-row">
                    <input
                      v-model="editSenderValue"
                      class="input senders__input"
                      type="text"
                      spellcheck="false"
                      autocomplete="off"
                      autofocus
                      :aria-invalid="Boolean(editSenderError)"
                      @input="editSenderError = null"
                      @keydown.esc.prevent="cancelEditSender"
                    />
                    <button
                      type="submit"
                      class="btn btn--sm btn--primary"
                      :disabled="!editSenderValue.trim()"
                    >
                      {{ t['settings.remote.save'] }}
                    </button>
                    <button type="button" class="btn btn--sm" @click="cancelEditSender">
                      {{ t['settings.remote.cancel'] }}
                    </button>
                  </div>
                  <p v-if="editSenderError" class="senders__error" role="alert">
                    {{ editSenderError }}
                  </p>
                </form>
                <div v-else class="senders__view">
                  <span class="senders__value" :title="sender">{{ sender }}</span>
                  <div class="senders__actions">
                    <UiIconAction
                      :label="t['settings.remote.edit']"
                      icon="ri:edit-2-line"
                      @click="beginEditSender(sender)"
                    />
                    <UiIconAction
                      :label="t['settings.remote.remove']"
                      icon="ri:delete-bin-5-line"
                      danger
                      @click="onRemoveSender(sender)"
                    />
                  </div>
                </div>
              </li>
            </ul>
          </UiScrollArea>
        </div>
      </div>
    </section>

    <!-- Spam gate. The verdict itself is server-side (`isSuspiciousMail`: a spam /
         Junk folder, or the server's own \Junk flag); this switch decides whether
         the reader keeps such a message folded until it is asked for. -->
    <section class="surface surface--spaced">
      <header class="surface__head">
        <h2>{{ t['settings.security.spam'] }}</h2>
      </header>

      <UiFieldRow
        :label="t['settings.security.spam_switch']"
        :hint="t['settings.security.spam_hint']"
      >
        <UiSwitch
          :checked="preferences.blockSuspiciousMail"
          @update:checked="onBlockSuspiciousChange"
        />
      </UiFieldRow>
    </section>
  </div>
</template>

<style scoped>
.row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 10px 18px;
}
/* Keep stacked content inside a ROW: the shared `.row__label` rule sets
   `flex: 0 0 240px`, which in a column container becomes a 200px-tall label box
   (a large blank gap). Use `.row--top` + a flex:1 content column instead. */
.row--top {
  align-items: flex-start;
}
.senders {
  flex: 1;
  min-width: 0;
}
.senders__add {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}
.senders__input {
  flex: 1;
  min-width: 0;
  height: 30px;
  font-size: 12.5px;
}
.proxy__input {
  width: 100%;
  height: 30px;
  font-size: 12.5px;
}
.proxy__error {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-danger);
}
.senders__error {
  margin: -4px 0 8px;
  font-size: 12px;
  color: var(--color-danger);
}
.senders__empty {
  margin: 0;
  font-size: 12px;
  opacity: 0.65;
}
.senders__scroll {
  max-height: 220px;
}
.senders__list {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.senders__item {
  min-width: 0;
  padding: 2px 0;
  font-size: 13px;
}
.senders__view {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
}
.senders__value {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.senders__actions {
  display: flex;
  flex: none;
  gap: 2px;
}
.senders__edit,
.senders__edit-row {
  display: flex;
  gap: 8px;
}
.senders__edit {
  flex-direction: column;
}
.senders__edit-row {
  align-items: center;
}
.senders__edit .senders__error {
  margin: 0;
}
</style>
