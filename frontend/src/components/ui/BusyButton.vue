<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { formatMessage } from '~/i18n/format'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    /** Idle label; also the prefix of the busy text unless `busyLabel` is set. */
    label: string
    /** Set while the action runs: disables the button and counts up. */
    busy?: boolean
    /** Custom busy text; `{s}` is replaced with the zero-padded elapsed seconds. */
    busyLabel?: string
    /** Widest text the button can show, used only to reserve width. */
    sizerLabel?: string
  }>(),
  { busy: false, busyLabel: '', sizerLabel: '' },
)

const { tf } = useI18n()

// Elapsed seconds while busy, so a slow or stuck action stays visible instead of
// collapsing into an ellipsis.
const seconds = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

watch(
  () => props.busy,
  (on) => {
    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
    seconds.value = 0
    if (on) {
      timer = setInterval(() => {
        seconds.value += 1
      }, 1000)
    }
  },
  { immediate: true },
)

onUnmounted(() => {
  if (timer !== null) clearInterval(timer)
})

// Zero-padded (and tabular-nums below) so the ticking counter cannot resize the
// button as it grows from 9s to 10s.
const elapsed = computed(() => String(seconds.value).padStart(2, '0'))

// The unit is part of the message, not a literal glued onto the label: English
// wants "12s", Chinese wants "12 秒", and the old `${label} {s}s` produced
// "打开浏览器授权 06s".
const busyText = computed(() =>
  props.busyLabel
    ? formatMessage(props.busyLabel, { s: elapsed.value })
    : tf('busy.seconds', { label: props.label, s: elapsed.value }),
)

const idleSizer = computed(() => props.sizerLabel || props.label)
</script>

<template>
  <button
    v-bind="$attrs"
    type="button"
    class="busy-btn"
    :disabled="busy || Boolean($attrs.disabled)"
  >
    <!-- Two hidden sizers keep the button as wide as its widest label, so
         switching to the busy text (and the seconds ticking) never resizes it. -->
    <span class="busy-btn__stack">
      <span class="busy-btn__cell busy-btn__sizer" aria-hidden="true">
        <slot />{{ idleSizer }}
      </span>
      <span class="busy-btn__cell busy-btn__sizer" aria-hidden="true">{{ busyText }}</span>
      <span class="busy-btn__cell">
        <template v-if="busy">{{ busyText }}</template>
        <template v-else><slot />{{ label }}</template>
      </span>
    </span>
  </button>
</template>

<style scoped>
/* The grid lives on our own wrapper, not on the <button>, so styles like
   `.btn { display: inline-flex }` keep working untouched. */
.busy-btn__stack {
  display: inline-grid;
  place-items: center;
  font-variant-numeric: tabular-nums;
}
.busy-btn__cell {
  grid-area: 1 / 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35em;
  white-space: nowrap;
}
.busy-btn__sizer {
  visibility: hidden;
}
</style>
