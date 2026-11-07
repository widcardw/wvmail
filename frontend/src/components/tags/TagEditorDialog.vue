<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { TagService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { useTagsStore } from '~/stores/tags'
import { TAG_FIELDS } from '~/types/tag'
import type { MailTag, TagMatchField } from '~/types/tag'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'
import UiColorPicker from '~/components/ui/UiColorPicker.vue'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'

const props = defineProps<{
  open: boolean
  tag: MailTag | null
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  saved: []
}>()

const { t } = useI18n()
const feedback = useFeedbackStore()
const tagsStore = useTagsStore()
const label = ref('')
const color = ref('#4f83cc')
const keywordsText = ref('')
const fields = ref<TagMatchField[]>(['subject', 'from'])
const enabled = ref(true)
const saving = ref(false)

const TAG_COLOR_PRESETS = [
  '#4f83cc',
  '#0891b2',
  '#0f766e',
  '#16a34a',
  '#ca8a04',
  '#ea580c',
  '#dc2626',
  '#db2777',
  '#9333ea',
  '#64748b',
]

const canSave = computed(
  () => label.value.trim() !== '' && parseKeywords().length > 0 && fields.value.length > 0,
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    label.value = props.tag?.label ?? ''
    color.value = props.tag?.color ?? '#4f83cc'
    keywordsText.value = props.tag?.keywords.join(', ') ?? ''
    fields.value = props.tag ? [...props.tag.fields] : ['subject', 'from']
    enabled.value = props.tag?.enabled ?? true
  },
)

function parseKeywords() {
  return [
    ...new Set(
      keywordsText.value
        .split(/[\n,，]+/)
        .map((item) => item.trim())
        .filter(Boolean),
    ),
  ]
}

function toggleField(field: TagMatchField, checked: boolean) {
  fields.value = checked
    ? TAG_FIELDS.filter((value) => value === field || fields.value.includes(value))
    : fields.value.filter((value) => value !== field)
}

async function save() {
  if (!canSave.value || saving.value) return
  saving.value = true
  try {
    await TagService.SaveMailTag({
      id: props.tag?.id ?? '',
      label: label.value.trim(),
      color: color.value,
      keywords: parseKeywords(),
      fields: fields.value,
      enabled: enabled.value,
      built_in: props.tag?.built_in ?? false,
      created_at: props.tag?.created_at ?? 0,
      updated_at: props.tag?.updated_at ?? 0,
    })
    await tagsStore.reload()
    feedback.success(t.value['tags.saved'])
    emit('saved')
    emit('update:open', false)
  } catch (error) {
    console.error('Failed to save tag', error)
    feedback.error(t.value['tags.saveFailed'])
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiDialog
    :open="open"
    :busy="saving"
    :close-on-interact-outside="false"
    @update:open="emit('update:open', $event)"
  >
    <UiDialogHeader
      :title="tag ? t['tags.edit'] : t['tags.add']"
      :close-label="t['cancel']"
      :disabled="saving"
      @close="emit('update:open', false)"
    />
    <div class="dlg-body">
      <label class="field">
        <span class="field__label">{{ t['tags.label'] }}</span>
        <input v-model="label" class="input" maxlength="64" placeholder="arXiv" />
      </label>

      <div class="field">
        <span class="field__label">{{ t['tags.color'] }}</span>
        <UiColorPicker
          variant="field"
          :label="t['tags.color']"
          :custom-label="t['tags.colorCustom']"
          :default-label="t['tags.colorDefault']"
          icon="i-ri:palette-line"
          :value="color"
          fallback="#4f83cc"
          :presets="TAG_COLOR_PRESETS"
          @select="color = $event"
          @reset="color = '#4f83cc'"
        />
      </div>

      <label class="field">
        <span class="field__label">{{ t['tags.keywords'] }}</span>
        <textarea
          v-model="keywordsText"
          class="input keyword-input"
          rows="4"
          placeholder="arxiv, kdd, aaai"
        />
        <span class="field__hint">{{ t['tags.keywordsHint'] }}</span>
      </label>

      <div class="field tag-fields-field">
        <span class="field__label">{{ t['tags.fields'] }}</span>
        <div class="field-grid">
          <UiCheckbox
            v-for="field in TAG_FIELDS"
            :key="field"
            class="checkbox-row"
            label-class="checkbox-row__label"
            :checked="fields.includes(field)"
            @update:checked="toggleField(field, $event)"
          >
            {{ t[`tags.field.${field}`] }}
          </UiCheckbox>
        </div>
      </div>

      <UiSwitch v-model:checked="enabled" class="switch-row">
        {{ t['tags.enabled'] }}
      </UiSwitch>
    </div>
    <footer class="dlg-foot">
      <button type="button" class="btn" :disabled="saving" @click="emit('update:open', false)">
        {{ t['cancel'] }}
      </button>
      <button type="button" class="btn btn--primary" :disabled="!canSave || saving" @click="save">
        {{ t['tags.save'] }}
      </button>
    </footer>
  </UiDialog>
</template>

<style scoped>
.keyword-input {
  height: auto;
  min-height: 82px;
  padding: 7px 8px;
  resize: vertical;
  font: inherit;
  font-size: 13px;
  line-height: 1.5;
}
.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px 14px;
}
.tag-fields-field {
  margin-bottom: 0;
}
.switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}
</style>
