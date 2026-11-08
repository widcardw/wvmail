<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { ScrollArea } from '@ark-ui/vue/scroll-area'

// One visual definition for every long page/list in the app. Ark supplies the
// scroll/keyboard/drag behaviour; the CSS hides the native scrollbar and draws a
// narrow 5px replacement. Call-site `class`/`style` belong on the root, which is
// also the flex child that has to shrink (`min-height: 0`) in a pane.

defineOptions({ inheritAttrs: false })

defineProps<{
  /** Show a horizontal scrollbar in addition to the vertical one. */
  horizontal?: boolean
  /** Props/events for Ark's scrollable viewport (for example useVirtualList). */
  viewportProps?: Record<string, unknown>
}>()

const attrs = useAttrs()
const rootClass = computed(() => attrs.class as string | undefined)
const rootStyle = computed(() => attrs.style as string | Record<string, string> | undefined)
</script>

<template>
  <ScrollArea.Root
    class="scroll-area"
    :class="[rootClass, { 'scroll-area--horizontal': horizontal }]"
    :style="rootStyle"
  >
    <ScrollArea.Viewport as-child>
      <!-- Keep the viewport as a real DOM node. Template refs supplied through
           `viewportProps` (notably useVirtualList) need scrollTop/clientHeight,
           not the Ark component instance. -->
      <div v-bind="viewportProps" class="scroll-area__viewport">
        <ScrollArea.Content class="scroll-area__content">
          <slot />
        </ScrollArea.Content>
      </div>
    </ScrollArea.Viewport>

    <ScrollArea.Scrollbar class="scroll-area__bar" orientation="vertical">
      <ScrollArea.Thumb class="scroll-area__thumb" />
    </ScrollArea.Scrollbar>

    <ScrollArea.Scrollbar v-if="horizontal" class="scroll-area__bar" orientation="horizontal">
      <ScrollArea.Thumb class="scroll-area__thumb" />
    </ScrollArea.Scrollbar>

    <ScrollArea.Corner v-if="horizontal" class="scroll-area__corner" />
  </ScrollArea.Root>
</template>

<style scoped>
.scroll-area {
  position: relative;
  display: block;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.scroll-area__viewport {
  display: block;
  height: 100%;
  /* A caller that only sets `max-height` leaves the root height auto, so the
     percentage above cannot form a scroll box by itself. Carry the cap onto the
     viewport as well; Ark's viewport already owns `overflow: auto`. */
  max-height: inherit;
  min-width: 0;
  min-height: 0;
  outline: none;
  scrollbar-width: none;
}
.scroll-area__viewport::-webkit-scrollbar {
  display: none;
}
.scroll-area__content {
  display: block;
  width: 100%;
}
/* Zag writes `min-width: fit-content` inline on the content so wide content can be
   scrolled horizontally. On a vertical-only pane that also sizes the content to its
   widest unbreakable child, which reaches the layout: a row's right-aligned date (or
   any nowrap text) lands outside the viewport and adds a horizontal scrollbar. Keep
   the content at the pane width unless the caller opted into horizontal scrolling;
   `!important` is needed because the value is inline. */
.scroll-area:not(.scroll-area--horizontal) .scroll-area__content {
  min-width: 0 !important;
}
.scroll-area__bar {
  z-index: 1;
  display: flex;
  box-sizing: border-box;
  padding: 3px;
  opacity: 0;
  visibility: hidden;
  pointer-events: none;
  transition:
    opacity 0.15s ease,
    visibility 0.15s ease;
}
.scroll-area__bar[data-orientation='vertical'] {
  position: absolute;
  top: 0;
  bottom: 0;
  right: 0;
  width: 7px;
  padding: 2px;
  justify-content: center;
}
.scroll-area__bar[data-orientation='horizontal'] {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 7px;
  padding: 2px;
  align-items: center;
}
.scroll-area__thumb {
  border-radius: 999px;
  background: color-mix(in srgb, var(--n-text-color) 26%, transparent);
  opacity: 0.55;
  transition:
    opacity 0.15s ease,
    background-color 0.15s ease;
}
.scroll-area__thumb[data-hover],
.scroll-area__thumb[data-dragging] {
  background: color-mix(in srgb, var(--n-text-color) 42%, transparent);
  opacity: 0.85;
}
.scroll-area__bar[data-orientation='vertical'] .scroll-area__thumb {
  width: 3px;
  min-height: 20px;
}
.scroll-area__bar[data-orientation='horizontal'] .scroll-area__thumb {
  height: 3px;
  min-width: 20px;
}

/* Show only when the content actually overflows and the pointer is inside the
   pane, or while the pane is actively scrolling/dragging. Deliberately not
   `:focus-within`: a clicked row keeps focus, which made the thumb linger after
   the pointer left. Ark removes `data-overflow-y/x` when there is nothing to
   scroll, so a non-overflowing pane never draws a thumb. */
.scroll-area:hover .scroll-area__bar[data-overflow-y],
.scroll-area:hover .scroll-area__bar[data-overflow-x],
.scroll-area__bar[data-overflow-y][data-scrolling],
.scroll-area__bar[data-overflow-x][data-scrolling],
.scroll-area__bar[data-overflow-y][data-dragging],
.scroll-area__bar[data-overflow-x][data-dragging] {
  opacity: 1;
  visibility: visible;
  pointer-events: auto;
}
.scroll-area__corner {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 9px;
  height: 9px;
}
</style>
