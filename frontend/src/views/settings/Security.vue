<script setup lang="ts">
import { useI18n } from '~/i18n'
import { usePreferencesStore } from '~/stores/preferences'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import UiFieldRow from '~/components/ui/UiFieldRow.vue'

const { t } = useI18n()
const preferences = usePreferencesStore()

function onAllowRemoteChange(checked: boolean) {
  void preferences.setAllowRemoteContent(checked)
}

function onBlockSuspiciousChange(checked: boolean) {
  void preferences.setBlockSuspiciousMail(checked)
}

function onRemoveSender(sender: string) {
  void preferences.removeAllowedRemoteSender(sender)
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

      <div class="row row--top">
        <span class="row__label">{{ t['settings.remote.hint'] }}</span>
        <div class="senders">
          <p v-if="preferences.allowedRemoteSenders.length === 0" class="senders__empty">
            {{ t['settings.remote.empty'] }}
          </p>
          <ul v-else class="senders__list">
            <li
              v-for="sender in preferences.allowedRemoteSenders"
              :key="sender"
              class="senders__item"
            >
              <span>{{ sender }}</span>
              <button type="button" class="btn btn--sm" @click="onRemoveSender(sender)">
                {{ t['settings.remote.remove'] }}
              </button>
            </li>
          </ul>
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
.senders__empty {
  margin: 0;
  font-size: 12px;
  opacity: 0.65;
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
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
