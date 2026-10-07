<script setup lang="ts">
import { Splitter } from '@ark-ui/vue/splitter'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useSearchStore } from '~/stores/search'
import EmailList from '~/components/list/EmailList.vue'
import EmailDetail from '~/components/detail/EmailDetail.vue'
import styles from '~/components/mail-split.module.css'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const searchStore = useSearchStore()

const active = () =>
  Boolean(accountsStore.selectedFolder || accountsStore.isSmartFolder || searchStore.searchActive)
</script>

<template>
  <div class="view-content">
    <Splitter.Root
      v-if="active()"
      :class="styles.root"
      :panels="[{ id: 'list', minSize: 18, maxSize: 55 }, { id: 'detail' }]"
      :default-size="[28, 72]"
    >
      <Splitter.Panel :class="styles.panel" id="list">
        <EmailList />
      </Splitter.Panel>

      <Splitter.ResizeTrigger :class="styles.trigger" id="list:detail" aria-label="resize split">
        <Splitter.ResizeTriggerIndicator :class="styles.indicator" />
      </Splitter.ResizeTrigger>

      <Splitter.Panel :class="styles.panel" id="detail">
        <EmailDetail />
      </Splitter.Panel>
    </Splitter.Root>

    <div v-else class="content-placeholder">
      <p class="ph-text">{{ t['mail.selectFolder'] }}</p>
    </div>
  </div>
</template>

<style scoped>
.view-content {
  height: calc(100vh - 40px);
  overflow: hidden;
}
.content-placeholder {
  max-width: 640px;
  margin: 0 auto;
}
.ph-text {
  padding: 4rem 0;
  text-align: center;
  opacity: 0.5;
  color: var(--n-text-color);
}
</style>
