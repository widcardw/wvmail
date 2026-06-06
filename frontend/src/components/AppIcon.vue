<script setup lang="ts">
import { computed } from 'vue'
import { Icon } from '@iconify/vue'
import { NIcon } from 'naive-ui'
import { getProviderIconUrl } from '../composables/provider-icons'

export type AppIconSource = 'iconify' | 'lobehub' | 'ben'

const props = withDefaults(
  defineProps<{
    /** Icon identifier: iconify name (e.g. 'ri:home-3-line'),
     *  provider key (e.g. 'openai'), or local image path */
    icon: string
    /** Icon source — 'iconify' (default), 'lobehub' (LLM provider avatars),
     *  or 'ben' (本地图片, local image path) */
    source?: AppIconSource
    /** Icon width/height in pixels */
    size?: number
    /** Border radius for the icon container */
    radius?: number
    /** Icon color (iconify only) */
    color?: string
    /** Background color behind the icon (e.g. '#FF3333', 'rgba(0,0,0,0.1)') */
    background?: string
  }>(),
  {
    source: 'iconify',
    size: 16,
    radius: 4,
  },
)

const imageUrl = computed(() => {
  if (props.source === 'lobehub') {
    return getProviderIconUrl(props.icon) ?? null
  }
  return null
})

const hasImage = computed(() => props.source === 'lobehub' && imageUrl.value !== null)

const containerStyle = computed(() => ({
  display: 'inline-flex' as const,
  alignItems: 'center' as const,
  justifyContent: 'center' as const,
  borderRadius: `${props.radius}px`,
  backgroundColor: props.background || undefined,
  padding: props.background ? '2px' : undefined,
  lineHeight: 1,
}))
</script>

<template>
  <span class="app-icon" :style="containerStyle">
    <!-- Iconify: NIcon + Icon -->
    <NIcon v-if="source === 'iconify'" :size="size" :color="color">
      <Icon :icon="icon" />
    </NIcon>

    <!-- LobeHub: provider avatar image -->
    <img
      v-else-if="source === 'lobehub' && hasImage"
      :src="imageUrl!"
      :width="size"
      :height="size"
      class="app-icon__img"
      :style="{ borderRadius: `${radius}px` }"
    />

    <!-- 本地图片 (local image path) -->
    <img
      v-else-if="source === 'ben'"
      :src="icon"
      :width="size"
      :height="size"
      class="app-icon__img"
      :style="{ borderRadius: `${radius}px` }"
    />
  </span>
</template>

<style scoped>
.app-icon {
  vertical-align: middle;
}

.app-icon__img {
  display: inline-block;
  vertical-align: middle;
}
</style>
