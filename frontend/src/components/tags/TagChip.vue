<script setup lang="ts">
import { computed } from 'vue'
import type { MailTag } from '~/types/tag'

const props = withDefaults(
  defineProps<{
    tag: MailTag
    compact?: boolean
  }>(),
  { compact: false },
)

const style = computed(() => ({
  color: props.tag.color,
  borderColor: `color-mix(in srgb, ${props.tag.color} 46%, transparent)`,
  background: `color-mix(in srgb, ${props.tag.color} 12%, transparent)`,
}))
</script>

<template>
  <span class="tag-chip" :class="{ 'tag-chip--compact': compact }" :style="style">
    <span class="tag-chip__dot" :style="{ background: tag.color }" />
    <span class="tag-chip__label">{{ tag.label }}</span>
  </span>
</template>

<style scoped>
.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  max-width: 180px;
  padding: 2px 7px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 11px;
  line-height: 1.4;
  white-space: nowrap;
}
.tag-chip--compact {
  max-width: 110px;
  padding: 1px 6px;
  font-size: 10px;
}
.tag-chip__dot {
  width: 6px;
  height: 6px;
  flex: none;
  border-radius: 999px;
}
.tag-chip__label {
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
