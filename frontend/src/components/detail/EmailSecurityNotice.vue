<script setup lang="ts">
import { useI18n } from '~/i18n'

// Spam/Junk gate: body and attachments stay hidden until the user asks for them. It is a
// body bar like the blocked-resource notices, so the message keeps its sender/subject
// header and the label truncates instead of breaking mid-phrase in a narrow pane (a CJK
// title inside a flex row wraps between any two characters, e.g. 不安 / 全).
const emit = defineEmits<{ reveal: [] }>()

const { t } = useI18n()
</script>

<template>
  <div class="body-bar body-bar--with-actions">
    <i class="i-ri:spam-2-line body-bar__icon" aria-hidden="true" />
    <span class="body-bar__text" :title="t('mail.security.body')">
      {{ t('mail.security.title') }}
    </span>
    <div class="body-bar__actions">
      <button type="button" class="btn btn--sm btn--subtle" @click="emit('reveal')">
        {{ t('mail.security.show_all') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
/* Geometry and tint live in styles/ui.css (.body-bar*); this component only owns text. */
</style>
