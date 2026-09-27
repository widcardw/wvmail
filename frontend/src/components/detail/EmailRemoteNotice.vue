<script setup lang="ts">
import { useI18n } from '~/i18n'

const props = defineProps<{
  count: number
  canAlwaysAllow?: boolean
}>()

const emit = defineEmits<{
  reveal: []
  always: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="remote-bar">
    <i class="i-ri:shield-keyhole-line bar-ic" aria-hidden="true" />
    <span
      class="bar-text"
      :title="t['mail.remote.blocked'].replace('{count}', String(props.count))"
    >
      {{ t['mail.remote.blocked_short'].replace('{count}', String(props.count)) }}
    </span>
    <div class="bar-actions">
      <button type="button" class="btn btn--sm btn--subtle" @click="emit('reveal')">
        {{ t['mail.remote.show'] }}
      </button>
      <button
        v-if="props.canAlwaysAllow"
        type="button"
        class="btn btn--sm btn--subtle"
        @click="emit('always')"
      >
        {{ t['mail.remote.always'] }}
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Mirrors EmailAttachments' .attach-head geometry so both bars read as one strip
 * between the header and the body; the tint comes from the shared warning tokens. */
.remote-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--n-border-color);
  background: var(--color-warning-bg);
  font-size: 13px;
  font-weight: 600;
  color: inherit;
}
.bar-ic {
  flex-shrink: 0;
  font-size: 16px;
  color: var(--color-warning);
}
.bar-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.bar-actions {
  display: flex;
  flex-shrink: 0;
  gap: 6px;
}
</style>
