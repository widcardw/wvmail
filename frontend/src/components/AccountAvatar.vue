<script setup lang="ts">
import { computed, h } from 'vue'
import { NIcon } from 'naive-ui'
import { ICON_MAPPING_SMALL as ICON_MAPPING } from '../composables/icon-mapping'
import { useAccountsStore } from '../stores/accounts'

const props = defineProps<{
  accountId: string
}>()

const store = useAccountsStore()

const account = computed(() => store.accounts.find((a) => a.ID === props.accountId))

const iconVNode = computed(() => {
  if (!account.value) return null
  const src = ICON_MAPPING[account.value.Provider]
  if (src) return src.create()
  return null
})

const fallbackLetter = computed(() => {
  if (iconVNode.value) return null
  return (account.value?.Name || account.value?.Email || '?').charAt(0).toUpperCase()
})
</script>

<template>
  <span class="account-avatar" v-if="iconVNode">
    <component :is="() => iconVNode" />
  </span>
  <span v-else class="account-avatar-fallback">{{ fallbackLetter }}</span>
</template>

<style scoped>
.account-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  line-height: 1;
  flex-shrink: 0;
}

.account-avatar-fallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  min-width: 16px;
  border-radius: 50%;
  background: var(--n-action-color);
  font-size: 8px;
  font-weight: 700;
  line-height: 1;
  color: var(--n-text-color);
}
</style>
