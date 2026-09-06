<script setup lang="ts">
import { computed } from 'vue'
import { getProviderIconUrl } from '../composables/provider-icons'

export type AppIconSource = 'iconify' | 'lobehub' | 'ben'

const props = withDefaults(
  defineProps<{
    /** Iconify icon (e.g. 'ri:home-3-line') — rendered as an offline UnoCSS
     *  `i-<icons>` icon; provider key for source='lobehub'; local path for 'ben'. */
    icon: string
    /** 'iconify' (default), 'lobehub' (LLM provider avatars), or 'ben' (本地图片) */
    source?: AppIconSource
    /** Icon size in px (drives the 1em font-size of the icon) */
    size?: number
    /** Border radius for background-backed icons/images */
    radius?: number
    /** Icon color (iconify — uses currentColor) */
    color?: string
    /** Background pill behind the icon (iconify) */
    background?: string
  }>(),
  {
    source: 'iconify',
    size: 16,
    radius: 4,
  },
)

const imageUrl = computed(() => {
  if (props.source === 'lobehub') return getProviderIconUrl(props.icon) ?? null
  return null
})
const hasImage = computed(() => props.source === 'lobehub' && imageUrl.value !== null)

// iconify class is applied to the inner <i> below; nothing needed on the wrapper.

const containerStyle = computed(() => ({
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: `${props.radius}px`,
  backgroundColor: props.background || undefined,
  padding: props.background ? '2px' : undefined,
  lineHeight: 1,
}))
const iconStyle = computed(() => ({
  fontSize: `${props.size}px`,
  color: props.color || undefined,
}))
</script>

<template>
  <span class="app-icon" :style="containerStyle">
    <!-- Offline iconify icon: UnoCSS emits .i-<name> for the inner <i>, colored via currentColor -->
    <i
      v-if="source === 'iconify'"
      class="app-icon__glyph"
      :class="{ [`i-${icon}`]: true }"
      :style="iconStyle"
      aria-hidden="true"
    />

    <!-- LobeHub provider avatar -->
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

.app-icon__glyph {
  line-height: 1;
}

.app-icon__img {
  display: inline-block;
  vertical-align: middle;
}
</style>
