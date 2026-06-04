<script setup lang="ts">
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import EmailList from '../components/EmailList.vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()
</script>

<template>
  <div class="view-content">
    <div v-if="accountsStore.selectedFolder" class="mail-content">
      <EmailList />
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
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.mail-content {
  height: 100%;
}

.content-placeholder {
  max-width: 640px;
  margin: 0 auto;
}
</style>
