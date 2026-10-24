<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { Tooltip } from '@ark-ui/vue/tooltip'
import { useI18n } from '~/i18n'
import { useConnectionStore } from '~/stores/connection'

// The meter is drawn from three CSS bars instead of an icon-font glyph: no preset
// icon carries a per-bar severity fill (three green vs. two amber vs. one red) that
// also lines up with a cross for "off". The tooltip is the only text, so its labels
// live here rather than in the title bar, and everything it shows comes from the
// connection store, which owns the polling schedule and the classification.
const { t, tf } = useI18n()
const store = useConnectionStore()

// Start on mount and stop on unmount even though the title bar lives for the
// whole session: it keeps the poller owned by whatever is showing it, so a second
// mount cannot double the probe rate.
onMounted(() => store.start())
onBeforeUnmount(() => store.stop())

const visible = computed(() => store.quality !== null || store.probing)

const statusLabel = computed(() => {
  switch (store.quality) {
    case 'High':
      return t.value['connection.high']
    case 'Medium':
      return t.value['connection.medium']
    case 'Low':
      return t.value['connection.low']
    case 'Off':
      return t.value['connection.off']
    default:
      return t.value['connection.checking']
  }
})

/** "imap.qq.com · 134 ms", or the reason it failed. */
const detail = computed(() => {
  if (store.error) return store.error
  if (!store.host) return ''
  const latency = store.latencyText
  return latency ? `${store.host} · ${latency}` : store.host
})

/** The retry is worth naming: it is the difference between slow and flaky. */
const retried = computed(() => store.attempts > 1)

const checkedAt = computed(() =>
  store.lastCheckedAt === 0
    ? ''
    : new Date(store.lastCheckedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
)

const ariaLabel = computed(() => [statusLabel.value, detail.value].filter(Boolean).join(' — '))
</script>

<template>
  <!-- Placement is bottom-end because the icon sits at the very top of the window;
       close-on-click is off so a manual re-check leaves the tooltip open while the
       latency updates in place. -->
  <Tooltip.Root
    v-if="visible"
    :open-delay="200"
    :close-delay="80"
    :close-on-click="false"
    :positioning="{ placement: 'bottom-end', gutter: 8 }"
  >
    <Tooltip.Trigger
      class="icon-btn meter-trigger"
      :data-quality="store.quality ?? 'Unknown'"
      :data-busy="store.probing"
      :aria-label="ariaLabel"
      @click="store.probe()"
    >
      <span class="meter" aria-hidden="true">
        <span
          v-for="bar in 3"
          :key="bar"
          class="meter__bar"
          :class="{ 'meter__bar--on': bar <= store.bars }"
          :style="{ height: `${2 + bar * 4}px` }"
        />
        <span class="meter__cross" />
      </span>
    </Tooltip.Trigger>
    <Teleport to="body">
      <Tooltip.Positioner class="meter-positioner">
        <Tooltip.Content class="meter-content">
          <span class="meter-content__title">{{ statusLabel }}</span>
          <span v-if="detail" class="meter-content__detail">{{ detail }}</span>
          <span v-if="retried" class="meter-content__note">{{ t['connection.retried'] }}</span>
          <span v-if="checkedAt" class="meter-content__note">
            {{ tf('connection.checked', { time: checkedAt }) }}
          </span>
          <span class="meter-content__hint">{{ t['connection.recheck'] }}</span>
        </Tooltip.Content>
      </Tooltip.Positioner>
    </Teleport>
  </Tooltip.Root>
</template>

<style scoped>
/* The wrapper stays out of the layout maths: the trigger is the icon-btn, with the
   padding reset its fixed 26px box wants. */
.meter-trigger {
  position: relative;
  padding: 0;
}

/* While a probe is in flight the bars breathe, so a manual re-check (click) and
   the 6s re-checks of a bad link are visible without changing the geometry. */
.meter-trigger[data-busy='true'] .meter {
  animation: meter-pulse 1.1s ease-in-out infinite;
}
@keyframes meter-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}
@media (prefers-reduced-motion: reduce) {
  .meter-trigger[data-busy='true'] .meter {
    animation: none;
  }
}

/* Geometry: three 4px bars stepping 6/10/14px, bottom-aligned inside a 16px box —
   the proportions of a signal meter (and the total width, 18px, is what the cross
   below is sized to). */
.meter {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 16px;
}

.meter__bar {
  width: 4px;
  border-radius: 1px;
  /* Bars that are not filled stay as a track, so the meter keeps its shape and
     the icon does not jump between states. */
  background: color-mix(in srgb, var(--n-text-color) 26%, transparent);
  transition: background-color 0.15s ease;
}

.meter-trigger[data-quality='High'] .meter__bar--on {
  background: var(--color-success);
}
.meter-trigger[data-quality='Medium'] .meter__bar--on {
  background: var(--color-warning);
}
.meter-trigger[data-quality='Low'] .meter__bar--on {
  background: var(--color-danger);
}

/* "Off" is the only state that overrides the bars: a cross drawn from two
   strokes over the empty track. */
.meter__cross {
  position: absolute;
  inset: 0;
  opacity: 0;
  transition: opacity 0.15s ease;
}
.meter-trigger[data-quality='Off'] .meter__cross {
  opacity: 1;
}
.meter__cross::before,
.meter__cross::after {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 18px;
  height: 2px;
  border-radius: 1px;
  background: var(--color-danger);
}
.meter__cross::before {
  transform: translate(-50%, -50%) rotate(45deg);
}
.meter__cross::after {
  transform: translate(-50%, -50%) rotate(-45deg);
}

/* --- Tooltip --- */
.meter-positioner {
  z-index: 1000;
}
.meter-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  max-width: 260px;
  padding: 6px 9px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.meter-content__title {
  font-weight: 600;
}
.meter-content__detail {
  color: var(--color-text-2);
  /* Host names and error text: wrap instead of widening the tooltip. */
  overflow-wrap: anywhere;
}
.meter-content__note,
.meter-content__hint {
  color: var(--color-text-3);
}
</style>
