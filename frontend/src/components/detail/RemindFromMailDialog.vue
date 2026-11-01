<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Field } from '@ark-ui/vue/field'
import { useI18n } from '~/i18n'
import { useRemindersStore } from '~/stores/reminders'
import { useFeedbackStore } from '~/stores/feedback'
import BusyButton from '~/components/ui/BusyButton.vue'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'
import { toLocalInputValue } from '~/composables/reminder-date'

// Confirmation step for a date/time detected inside an email body. The detector is
// allowed to be approximate precisely because this dialog exists: the moment can be
// corrected (or the whole thing cancelled) before anything is stored.
const props = defineProps<{
  open: boolean
  /** Detected moment, in epoch seconds. */
  at: number
  /** The text the detector matched, shown so the user can judge the guess. */
  text: string
  /** Timezone named in that text, when the detector had to convert from it. */
  zone: string
  /** Suggested reminder title — the mail's subject. */
  suggestedTitle: string
}>()

const emit = defineEmits<{ 'update:open': [open: boolean] }>()

const { t, tf } = useI18n()
const reminders = useRemindersStore()
const fb = useFeedbackStore()

const title = ref('')
const time = ref('')
const saving = ref(false)

// Re-seed on every open: the next mention belongs to another moment, and often to
// another email.
watch(
  () => props.open,
  (open) => {
    if (!open) return
    title.value = props.suggestedTitle.trim()
    time.value = toLocalInputValue(new Date(props.at * 1000))
  },
  { immediate: true },
)

const canSave = computed(() => title.value.trim().length > 0 && time.value.length > 0)

/** The detector accepts moments around the mail's date, so a chip can point at the past. */
const isPast = computed(() => {
  const at = new Date(time.value).getTime()
  return !Number.isNaN(at) && at < Date.now()
})

function close() {
  emit('update:open', false)
}

async function save() {
  if (!canSave.value || saving.value) return
  const at = Math.floor(new Date(time.value).getTime() / 1000)
  if (Number.isNaN(at)) return
  saving.value = true
  try {
    const saved = await reminders.add(title.value, at)
    if (!saved) {
      fb.error(t.value['mail.remind.failed'])
      return
    }
    fb.success(t.value['mail.remind.added'])
    close()
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiDialog :open="props.open" @update:open="emit('update:open', $event)">
    <UiDialogHeader :title="t['mail.remind.title']" :close-label="t['cancel']" @close="close" />

    <div class="dlg-body">
      <Field.Root class="field" :disabled="true">
        <Field.Label class="field__label">{{ t['mail.remind.body'] }}</Field.Label>
        <Field.Input class="ctl" :model-value="props.text" />
      </Field.Root>
      <Field.Root class="field">
        <Field.Label class="field__label">{{ t['mail.remind.field.title'] }}</Field.Label>
        <Field.Input
          class="ctl"
          :model-value="title"
          @update:model-value="title = String($event)"
        />
      </Field.Root>
      <Field.Root class="field">
        <Field.Label class="field__label">{{ t['mail.remind.field.time'] }}</Field.Label>
        <Field.Input v-model="time" class="ctl datetime" type="datetime-local" />
        <Field.HelperText v-if="props.zone" class="field__hint">
          {{ tf('mail.remind.zone', { zone: props.zone }) }}
        </Field.HelperText>
        <Field.HelperText v-if="isPast" class="field__hint field__hint--warn">
          {{ t['mail.remind.past'] }}
        </Field.HelperText>
      </Field.Root>
    </div>

    <footer class="dlg-foot">
      <button type="button" class="btn" @click="close">{{ t['cancel'] }}</button>
      <BusyButton
        class="btn btn--primary"
        :label="t['mail.remind.add']"
        :busy="saving"
        :disabled="!canSave"
        @click="save"
      />
    </footer>
  </UiDialog>
</template>
