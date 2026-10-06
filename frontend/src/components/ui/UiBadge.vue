<script setup lang="ts">
import { computed } from 'vue'
import { formatBadgeCount } from '~/composables/badge'

// Ark UI ships no Badge (there is no `@ark-ui/vue/badge`), so this is the app's own thin
// wrapper: presentational only, geometry and tint live in styles/ui.css (`.badge`).
//
// The visual text is abbreviated (`99+`), so the real count rides along as the
// accessible name; `role="img"` is what makes `aria-label` the name of a bare <span>.
const props = withDefaults(
  defineProps<{
    /** Raw count. Zero or less renders nothing at all. */
    count: number
    /** Counts above this are shown as `{max}+` (99 by default). */
    max?: number
    /** Accessible name (and tooltip), e.g. "5 unread". Defaults to the visible text. */
    label?: string
  }>(),
  { max: 99, label: '' },
)

const text = computed(() => formatBadgeCount(props.count, props.max))
const name = computed(() => props.label || text.value)
</script>

<template>
  <span v-if="text" class="badge" role="img" :aria-label="name" :title="name">{{ text }}</span>
</template>
