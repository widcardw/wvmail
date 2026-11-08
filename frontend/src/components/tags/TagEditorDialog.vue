<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { TagsInput } from '@ark-ui/vue/tags-input'
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
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
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
const keywords = ref<string[]>([])
const fields = ref<TagMatchField[]>(['subject', 'from'])
const enabled = ref(true)
const saving = ref(false)
const keywordDelimiter = /[,\n，]/

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
  () =>
    label.value.trim() !== '' &&
    keywords.value.some((keyword) => keyword.trim()) &&
    fields.value.length > 0,
)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    label.value = props.tag?.label ?? ''
    color.value = props.tag?.color ?? '#4f83cc'
    keywords.value = props.tag ? [...props.tag.keywords] : []
    fields.value = props.tag ? [...props.tag.fields] : ['subject', 'from']
    enabled.value = props.tag?.enabled ?? true
  },
)

function sanitizeKeyword(value: string) {
  return value.trim()
}

function parseKeywords() {
  return [...new Set(keywords.value.map(sanitizeKeyword).filter(Boolean))]
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
    feedback.success(t('tags.saved'))
    emit('saved')
    emit('update:open', false)
  } catch (error) {
    console.error('Failed to save tag', error)
    feedback.error(t('tags.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiDialog
    :open="open"
    content-class="tag-editor-dialog"
    :busy="saving"
    :close-on-interact-outside="false"
    @update:open="emit('update:open', $event)"
  >
    <UiDialogHeader
      :title="tag ? t('tags.edit') : t('tags.add')"
      :close-label="t('cancel')"
      :disabled="saving"
      @close="emit('update:open', false)"
    />
    <div class="dlg-body tag-editor-body">
      <UiScrollArea class="tag-editor-scroll">
        <label class="field">
          <span class="field__label">{{ t('tags.label') }}</span>
          <input v-model="label" class="input" maxlength="64" placeholder="arXiv" />
        </label>

        <div class="field">
          <span class="field__label">{{ t('tags.color') }}</span>
          <UiColorPicker
            variant="field"
            :label="t('tags.color')"
            :custom-label="t('tags.colorCustom')"
            :default-label="t('tags.colorDefault')"
            icon="i-ri:palette-line"
            :value="color"
            fallback="#4f83cc"
            :presets="TAG_COLOR_PRESETS"
            @select="color = $event"
            @reset="color = '#4f83cc'"
          />
        </div>

        <div class="field">
          <TagsInput.Root
            v-model="keywords"
            class="keyword-tags"
            :delimiter="keywordDelimiter"
            :sanitize-value="sanitizeKeyword"
            blur-behavior="add"
            add-on-paste
            :disabled="saving"
          >
            <TagsInput.Label class="field__label">{{ t('tags.keywords') }}</TagsInput.Label>
            <TagsInput.Context v-slot="tagsInput">
              <TagsInput.Control class="keyword-control">
                <TagsInput.Item
                  v-for="(value, index) in tagsInput.value"
                  :key="index"
                  :index="index"
                  :value="value"
                  class="keyword-item"
                >
                  <TagsInput.ItemPreview class="keyword-item-preview">
                    <TagsInput.ItemText class="keyword-item-text">{{ value }}</TagsInput.ItemText>
                    <TagsInput.ItemDeleteTrigger class="keyword-delete" :disabled="saving">
                      <i class="i-ri:close-line" aria-hidden="true" />
                    </TagsInput.ItemDeleteTrigger>
                  </TagsInput.ItemPreview>
                  <TagsInput.ItemInput class="keyword-item-input" />
                </TagsInput.Item>
                <TagsInput.Input
                  class="keyword-input"
                  :placeholder="t('tags.keywordPlaceholder')"
                />
              </TagsInput.Control>
            </TagsInput.Context>
            <TagsInput.HiddenInput />
          </TagsInput.Root>
          <span class="field__hint">{{ t('tags.keywordsHint') }}</span>
        </div>

        <div class="field tag-fields-field">
          <span class="field__label">{{ t('tags.fields') }}</span>
          <div class="field-grid">
            <UiCheckbox
              v-for="field in TAG_FIELDS"
              :key="field"
              class="checkbox-row"
              label-class="checkbox-row__label"
              :checked="fields.includes(field)"
              @update:checked="toggleField(field, $event)"
            >
              {{ t(`tags.field.${field}`) }}
            </UiCheckbox>
          </div>
        </div>

        <UiSwitch v-model:checked="enabled" class="switch-row">
          {{ t('tags.enabled') }}
        </UiSwitch>
      </UiScrollArea>
    </div>
    <footer class="dlg-foot">
      <button type="button" class="btn" :disabled="saving" @click="emit('update:open', false)">
        {{ t('cancel') }}
      </button>
      <button type="button" class="btn btn--primary" :disabled="!canSave || saving" @click="save">
        {{ t('tags.save') }}
      </button>
    </footer>
  </UiDialog>
</template>

<style scoped>
:global(.tag-editor-dialog) {
  max-height: min(760px, calc(100vh - 32px));
}
.tag-editor-body {
  padding: 0;
  overflow: hidden;
}
.tag-editor-scroll {
  height: 100%;
  padding: 16px;
}
.keyword-tags {
  width: 100%;
}
.keyword-control {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-height: 40px;
  padding: 6px 8px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.keyword-control:focus-within {
  border-color: var(--n-primary-color);
}
.keyword-item,
.keyword-item-preview {
  display: inline-flex;
  align-items: center;
}
.keyword-item {
  max-width: 100%;
  gap: 5px;
  padding: 3px 6px;
  overflow: hidden;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 5px;
  font-size: 12px;
}
.keyword-item-preview {
  max-width: 100%;
  gap: 5px;
}
.keyword-item-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.keyword-delete {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  padding: 0;
  color: var(--color-text-3);
  background: transparent;
  border: 0;
  cursor: pointer;
}
.keyword-delete:hover {
  color: var(--color-danger);
}
.keyword-input,
.keyword-item-input {
  min-width: 100px;
  min-height: 24px;
  padding: 0;
  color: var(--n-text-color);
  background: transparent;
  border: 0;
  outline: 0;
  font: inherit;
  font-size: 13px;
}
.keyword-input {
  flex: 1;
}
.keyword-input::placeholder {
  color: var(--color-text-3);
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
