<script setup lang="ts">
import { ref, watch } from 'vue'
import { TagService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { useTagsStore } from '~/stores/tags'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'

const props = defineProps<{
  open: boolean
  accountId: string
  folder: string
  uid: number | null
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  saved: [tagIds: string[]]
}>()

const { t } = useI18n()
const feedback = useFeedbackStore()
const tagsStore = useTagsStore()
const selected = ref<string[]>([])
const automatic = ref<string[]>([])
const loading = ref(false)
const saving = ref(false)

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    selected.value = []
    automatic.value = []
    loading.value = true
    try {
      if (tagsStore.tags.length === 0) await tagsStore.loadTags()
      if (props.uid == null || !props.accountId || !props.folder) return
      const state = await TagService.GetEmailTagState(props.accountId, props.folder, props.uid)
      selected.value = [...state.selected_tag_ids]
      automatic.value = [...state.automatic_tag_ids]
    } catch (error) {
      console.error('Failed to load message tags', error)
      feedback.error(t.value['mail.tags.failed'])
    } finally {
      loading.value = false
    }
  },
)

function toggle(id: string, checked: boolean) {
  selected.value = checked
    ? [...new Set([...selected.value, id])]
    : selected.value.filter((value) => value !== id)
}

async function save() {
  if (saving.value || props.uid == null) return
  saving.value = true
  try {
    await TagService.SetEmailTags(props.accountId, props.folder, props.uid, selected.value)
    tagsStore.setEmailTagIDs(props.accountId, props.folder, props.uid, [...selected.value])
    feedback.success(t.value['mail.tags.saved'])
    emit('saved', [...selected.value])
    emit('update:open', false)
  } catch (error) {
    console.error('Failed to save message tags', error)
    feedback.error(t.value['mail.tags.failed'])
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
      :title="t['mail.tags.title']"
      :close-label="t['cancel']"
      :disabled="saving"
      @close="emit('update:open', false)"
    />
    <div class="dlg-body">
      <div v-if="loading" class="tag-loading"><span class="spinner" /></div>
      <p v-else-if="tagsStore.tags.length === 0" class="tag-empty">
        {{ t['tags.noTagsYet'] }}
      </p>
      <div v-else class="tag-list">
        <UiCheckbox
          v-for="tag in tagsStore.tags"
          :key="tag.id"
          class="tag-row"
          label-class="tag-row__label"
          :checked="selected.includes(tag.id)"
          @update:checked="toggle(tag.id, $event)"
        >
          <span class="tag-dot" :style="{ background: tag.color }" />
          <span class="tag-name">{{ tag.label }}</span>
          <span v-if="automatic.includes(tag.id)" class="tag-auto">
            {{ t['mail.tags.automatic'] }}
          </span>
          <span v-if="!tag.enabled" class="tag-disabled">{{ t['tags.enabled'] }}</span>
        </UiCheckbox>
      </div>
    </div>
    <footer class="dlg-foot">
      <button type="button" class="btn" :disabled="saving" @click="emit('update:open', false)">
        {{ t['cancel'] }}
      </button>
      <button
        type="button"
        class="btn btn--primary"
        :disabled="saving || loading || tagsStore.tags.length === 0"
        @click="save"
      >
        {{ t['tags.save'] }}
      </button>
    </footer>
  </UiDialog>
</template>

<style scoped>
.tag-loading {
  display: flex;
  justify-content: center;
  padding: 28px;
}
.tag-empty {
  margin: 0;
  padding: 20px 0;
  color: var(--color-text-3);
  font-size: 13px;
  text-align: center;
}
.tag-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.tag-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  border-radius: 6px;
  font-size: 13px;
}
.tag-row:hover {
  background: var(--n-sidebar-hover);
}
.tag-row__label {
  display: flex;
  align-items: center;
  gap: 7px;
  flex: 1;
  min-width: 0;
}
.tag-dot {
  width: 9px;
  height: 9px;
  flex: none;
  border-radius: 999px;
}
.tag-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tag-auto,
.tag-disabled {
  margin-left: auto;
  color: var(--color-text-3);
  font-size: 10px;
}
.tag-auto {
  color: var(--n-primary-color);
}
</style>
