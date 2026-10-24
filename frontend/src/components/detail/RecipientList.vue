<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { layoutRecipients } from '~/composables/recipients'
import type { RecipientLayout } from '~/composables/recipients'

// One To/Cc row of the reading pane: a label, the addresses, and — when they do
// not fit on one line — a chip standing for the rest.
//
// Not Ark UI's Collapsible: that hides content behind a trigger with a fixed
// height, but the trigger here is *derived* from the measured layout (how many
// addresses the first line holds), so the cut has to be computed anyway. What is
// shared with the rest of the app is the behaviour: one line by default, the chip
// toggles both ways, nothing is hidden without saying how much.
const props = defineProps<{
  label: string
  addresses: string[]
}>()

const { t, tf, locale } = useI18n()

const box = ref<HTMLElement | null>(null)
const probe = ref<HTMLElement | null>(null)
const expanded = ref(false)
// null = not measured yet, so every address is shown and there is no chip. That
// is also what a hidden pane gets (clientWidth 0), which is better than an empty row.
const layout = ref<RecipientLayout | null>(null)

const total = computed(() => props.addresses.length)
/** The list does not fit on one line, so there is a chip at all. */
const collapsible = computed(() => (layout.value?.hidden ?? 0) > 0)
/** How many the chip currently stands for; 0 once expanded. */
const hidden = computed(() => (expanded.value ? 0 : (layout.value?.hidden ?? 0)))
const visible = computed(() => {
  const count = expanded.value || !layout.value ? total.value : layout.value.visible
  return props.addresses.slice(0, count)
})

/**
 * Measure the off-screen copy and decide the cut. The probe holds every address
 * at its natural width plus a chip sized for the worst case (the count of *all*
 * addresses, so the reservation never depends on the answer it is computing).
 *
 * The 2px shave covers `offsetWidth` rounding: it reports whole pixels per item,
 * and the collapsed row clips its overflow, so a half-pixel of optimism would cut
 * the last glyph off the chip.
 */
function measure() {
  const boxEl = box.value
  const probeEl = probe.value
  if (!boxEl || !probeEl) return
  const items = Array.from(probeEl.querySelectorAll<HTMLElement>('[data-item]'))
  const chip = probeEl.querySelector<HTMLElement>('[data-chip]')
  const sep = probeEl.querySelector<HTMLElement>('[data-sep]')
  layout.value = layoutRecipients(
    items.map((el) => el.offsetWidth),
    boxEl.clientWidth - 2,
    chip?.offsetWidth ?? 0,
    sep?.offsetWidth ?? 0,
  )
}

// Only a *width* change can change the answer; the row's own height changes when
// the cut moves, and reacting to that would loop.
let observer: ResizeObserver | null = null
let lastWidth = -1

onMounted(() => {
  measure()
  if (typeof ResizeObserver === 'undefined' || !box.value) return
  lastWidth = box.value.clientWidth
  observer = new ResizeObserver(() => {
    const width = box.value?.clientWidth ?? 0
    if (width === lastWidth) return
    lastWidth = width
    measure()
  })
  observer.observe(box.value)
})

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})

// Another message starts collapsed. Comparing the content rather than the array
// identity keeps a parent that rebuilds the array on every render from snapping an
// expanded list shut under the user's cursor.
watch(
  () => props.addresses,
  (next, prev) => {
    const changed =
      !prev || next.length !== prev.length || next.some((address, i) => address !== prev[i])
    if (!changed) return
    expanded.value = false
    nextTick(measure)
  },
)

// The chip is translated, so its width changes with the language.
watch(locale, () => nextTick(measure))
</script>

<template>
  <div class="mrow">
    <span class="mlabel">{{ label }}</span>
    <div ref="box" class="mvalue" :class="{ 'mvalue--clipped': hidden > 0 }">
      <template v-for="(address, i) in visible" :key="`${i}-${address}`">
        <span class="addr">{{ address }}</span>
        <span v-if="i < visible.length - 1" class="sep">, </span>
      </template>
      <button
        v-if="collapsible"
        type="button"
        class="chip"
        :aria-expanded="expanded"
        @click="expanded = !expanded"
      >
        {{ expanded ? t['mail.recipients.less'] : tf('mail.recipients.more', hidden) }}
      </button>
    </div>
    <div ref="probe" class="probe" aria-hidden="true">
      <template v-for="(address, i) in addresses" :key="`probe-${i}`">
        <span data-item class="addr">{{ address }}</span>
        <span v-if="i < addresses.length - 1" data-sep class="sep">, </span>
      </template>
      <span data-chip class="chip">{{ tf('mail.recipients.more', total) }}</span>
    </div>
  </div>
</template>

<style scoped>
.mrow {
  display: flex;
  /* Align the label with the first address, not with the top of its box: the
     label and the value used to inherit different line-heights (normal vs 1.6),
     which put the label's text 1px higher than the addresses. */
  align-items: baseline;
  gap: 6px;
  margin-bottom: 2px;
}
.mlabel {
  font-weight: 600;
  min-width: 26px;
  flex-shrink: 0;
  /* Same as the value's, so the label's own box is exactly one line tall. */
  line-height: 1.6;
  opacity: 0.6;
}
.mvalue {
  flex: 1 1 auto;
  min-width: 0;
  line-height: 1.6;
}
/* Collapsed: exactly one line, whatever the measurement came up with. */
.mvalue--clipped {
  white-space: nowrap;
  overflow: hidden;
}
.addr {
  word-break: break-word;
  opacity: 0.85;
}
.sep {
  opacity: 0.6;
}
.chip {
  border: 0;
  background: none;
  padding: 0 2px;
  margin-left: 2px;
  font: inherit;
  color: var(--n-primary-color);
  cursor: pointer;
  white-space: nowrap;
}
.chip:hover {
  text-decoration: underline;
}
.chip:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
  border-radius: 3px;
}
/* The measuring copy: laid out for real (offsetWidth needs that) but invisible and
   out of flow, so it never costs a line or a scrollbar. */
.probe {
  position: absolute;
  left: -9999px;
  top: 0;
  width: max-content;
  visibility: hidden;
  white-space: nowrap;
  pointer-events: none;
}
</style>
