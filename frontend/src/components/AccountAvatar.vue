<script setup lang="ts">
import { computed, h, ref, watch } from 'vue'
import { NIcon } from 'naive-ui'
import { ICON_MAPPING_SMALL as ICON_MAPPING } from '../composables/icon-mapping'
import { useAccountsStore } from '../stores/accounts'
import { ConfigService } from '../../bindings/wvmail/services'

const props = defineProps<{
  accountId: string
  size?: number
}>()

const store = useAccountsStore()

const account = computed(() => store.accounts.find((a) => a.ID === props.accountId))
const avatarSrc = ref<string | null>(null)

// Load avatar from config dir when account has Avatar filename
watch(
  () => account.value?.Avatar,
  async (avatarFile) => {
    avatarSrc.value = null
    if (!avatarFile || !props.accountId) return
    try {
      const uri = await ConfigService.GetAccountAvatar(props.accountId)
      avatarSrc.value = uri || null
    } catch (e) {
      console.error('Failed to load avatar', e)
      avatarSrc.value = null
    }
  },
  { immediate: true },
)

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
  <img
    v-if="avatarSrc"
    :src="avatarSrc"
    class="account-avatar account-avatar-img"
    :style="{ width: size + 'px', height: size + 'px' }"
  />
  <span class="account-avatar" v-else-if="iconVNode">
    <component :is="() => iconVNode" />
  </span>
  <span
    v-else
    class="account-avatar-fallback"
    :style="{ width: size + 'px', height: size + 'px' }"
    >{{ fallbackLetter }}</span
  >
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

.account-avatar-img {
  border-radius: 6px;
  object-fit: cover;
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
