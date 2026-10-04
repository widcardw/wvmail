<script setup lang="ts">
import { useI18n } from '~/i18n'

// A message whose body is drawn with CSS (one `box-shadow` entry per pixel, or any other
// oversized style value) has no text and no images to show once the styling is dropped —
// that is exactly the shape of image spam. Say so instead of rendering an empty body.
const props = defineProps<{ count: number }>()

const { t } = useI18n()
</script>

<template>
  <div class="body-bar">
    <i class="i-ri:brush-line body-bar__icon" aria-hidden="true" />
    <span
      class="body-bar__text"
      :title="t['mail.cssArt.blocked'].replace('{count}', String(props.count))"
    >
      {{ t['mail.cssArt.blocked_short'].replace('{count}', String(props.count)) }}
    </span>
  </div>
</template>

<style scoped>
/* Geometry and tint live in styles/ui.css (.body-bar*); this component only owns text. */
</style>
