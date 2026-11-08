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
  <div class="body-bar body-bar--with-actions">
    <i class="i-ri:shield-keyhole-line body-bar__icon" aria-hidden="true" />
    <!-- `count` is passed as a number, so the message picks its singular or plural
         branch and can still print the number via `{count}`. -->
    <span class="body-bar__text" :title="t('mail.remote.blocked', props.count)">
      {{ t('mail.remote.blocked_short', props.count) }}
    </span>
    <div class="body-bar__actions">
      <button type="button" class="btn btn--sm btn--subtle" @click="emit('reveal')">
        {{ t('mail.remote.show') }}
      </button>
      <button
        v-if="props.canAlwaysAllow"
        type="button"
        class="btn btn--sm btn--subtle"
        @click="emit('always')"
      >
        {{ t('mail.remote.always') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Geometry and tint live in styles/ui.css (.body-bar*); this component only owns text. */
</style>
