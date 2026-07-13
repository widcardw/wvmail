<script setup lang="ts">
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import EmailList from '~/components/EmailList.vue'
import EmailDetail from '~/components/EmailDetail.vue'
import { NSplit } from 'naive-ui'

const { t } = useI18n()
const accountsStore = useAccountsStore()
</script>

<template>
  <div class="view-content">
    <NSplit v-if="accountsStore.selectedFolder || accountsStore.isSmartFolder" 
    :max="0.75" :min="0.25" :default-size="0.25"
    :resize-trigger-size="2">
      <template #1>
        <EmailList />
      </template>
      <template #2>
        <EmailDetail />
      </template>
    </NSplit>
    <div v-else class="content-placeholder">
      <p style="opacity: 0.5; text-align: center; padding: 4rem 0">
        {{ t['mail.selectFolder'] }}
      </p>
    </div>
  </div>
</template>

<style scoped>
.view-content {
  height: calc(100vh - 41px);
  overflow: hidden;
}

.mail-split {
  height: 100%;
  display: flex;
  flex-direction: row;
}

.split-list {
  width: 340px;
  height: 100%;
  flex-shrink: 0;
  border-right: 1px solid var(--n-border-color);
}

.split-detail {
  flex: 1;
  height: 100%;
  min-width: 0;
}

.content-placeholder {
  max-width: 640px;
  margin: 0 auto;
  padding: 16px;
}
</style>
