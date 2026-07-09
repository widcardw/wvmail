<script setup lang="ts">
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import EmailList from '~/components/EmailList.vue'
import EmailDetail from '~/components/EmailDetail.vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()
</script>

<template>
  <div class="view-content">
    <div v-if="accountsStore.selectedFolder || accountsStore.isSmartFolder" class="mail-split">
      <div class="split-list">
        <EmailList />
      </div>
      <div class="split-detail">
        <EmailDetail />
      </div>
    </div>
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
