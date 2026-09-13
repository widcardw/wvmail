<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Avatar } from '@ark-ui/vue/avatar'
import { ICON_MAPPING_SMALL as ICON_MAPPING } from '~/composables/icon-mapping'
import { useAccountsStore } from '~/stores/accounts'
import { ConfigService } from '#/wvmail/services'

const props = withDefaults(
  defineProps<{
    accountId: string
    size?: number
  }>(),
  { size: 16 },
)

const store = useAccountsStore()

const account = computed(() => store.accounts.find((a) => a.ID === props.accountId))
const avatarSrc = ref<string | null>(null)
const loadKey = ref(0)
const avatarRevision = computed(() => store.avatarRevision[props.accountId] ?? 0)

watch(
  [() => props.accountId, () => account.value?.Avatar, avatarRevision],
  async (avatarFile) => {
    const requestKey = ++loadKey.value
    avatarSrc.value = null
    if (!avatarFile[1] || !props.accountId) return
    try {
      const uri = await ConfigService.GetAccountAvatar(props.accountId)
      if (requestKey === loadKey.value) avatarSrc.value = uri || null
    } catch {
      if (requestKey === loadKey.value) avatarSrc.value = null
    }
  },
  { immediate: true },
)

// Small rounded-corner style (Ark Avatar default would be rounder / different).
const avatarStyle = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  borderRadius: `${Math.max(2, Math.round(props.size * 0.3))}px`,
}))

const displayName = computed(
  () => account.value?.Name || account.value?.Email || account.value?.Avatar || '?',
)
// internal "prebuilt" avatar source for the provider (or initials fallback)
const glyph = computed(() => {
  if (!account.value) return null
  const src = ICON_MAPPING[account.value.Provider]
  return src ? src.create() : null
})
const fallback = computed(() => {
  if (glyph.value) return null
  return (account.value?.Name || account.value?.Email || '?').charAt(0).toUpperCase()
})
</script>

<template>
  <!-- Mutually exclusive: an authored fallback is only rendered while there is NO
       verified avatar image. Once avatarSrc exists we show only the Image and remove
       the Fallback so a provider initial/letter never flashes in front of the image. -->
  <Avatar.Root class="aa" :style="avatarStyle">
    <Avatar.Image
      v-if="avatarSrc"
      class="aa__img"
      :src="avatarSrc"
      :alt="displayName"
      @error="avatarSrc = null"
    />
    <Avatar.Fallback
      v-else
      class="aa__fb"
      :style="avatarStyle"
      :key="account ? account.ID : 'none'"
    >
      <template v-if="glyph"><component :is="() => glyph" /></template>
      <template v-else>{{ fallback }}</template>
    </Avatar.Fallback>
  </Avatar.Root>
</template>

<style scoped>
.aa {
  display: inline-flex;
  flex-shrink: 0;
  overflow: hidden;
  line-height: 1;
}
.aa__fb {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  background: var(--n-action-color);
  font-size: max(8px, 1em);
  color: var(--n-text-color);
}
.aa__img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: inherit;
}
</style>
