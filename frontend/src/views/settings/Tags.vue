<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { TagService } from '#/wvmail/services'
import { useI18n, type TranslationSchema } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'
import { useTagsStore } from '~/stores/tags'
import type { MailTag } from '~/types/tag'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'
import TagEditorDialog from '~/components/tags/TagEditorDialog.vue'
import TagSuggestionDialog from '~/components/tags/TagSuggestionDialog.vue'

const { t, tf } = useI18n()
const feedback = useFeedbackStore()
const tagsStore = useTagsStore()
const editorOpen = ref(false)
const editing = ref<MailTag | null>(null)
const aiOpen = ref(false)
const deleteTarget = ref<MailTag | null>(null)
const deleting = ref(false)

onMounted(() => tagsStore.loadTags())

function openAdd() {
  editing.value = null
  editorOpen.value = true
}

function openEdit(tag: MailTag) {
  editing.value = tag
  editorOpen.value = true
}

function fieldLabel(field: string): string {
  return t.value[`tags.field.${field}` as keyof TranslationSchema] ?? field
}

async function confirmDelete() {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  try {
    await TagService.DeleteMailTag(deleteTarget.value.id)
    await tagsStore.reload()
    feedback.success(t.value['tags.deleted'])
    deleteTarget.value = null
  } catch (error) {
    console.error('Failed to delete tag', error)
    feedback.error(t.value['tags.deleteFailed'])
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div class="wrap">
    <section class="surface">
      <header class="surface__head">
        <div>
          <h2>{{ t['tags.title'] }}</h2>
          <p class="tag-subtitle">{{ t['tags.subtitle'] }}</p>
        </div>
        <div class="head-actions">
          <button type="button" class="btn btn--sm" @click="aiOpen = true">
            <i class="i-ri:sparkling-2-line" aria-hidden="true" />
            {{ t['tags.ai.open'] }}
          </button>
          <button type="button" class="btn btn--primary btn--sm" @click="openAdd">
            <i class="i-ri:add-line" aria-hidden="true" />
            {{ t['tags.add'] }}
          </button>
        </div>
      </header>

      <p v-if="tagsStore.tagsLoading" class="tag-state"><span class="spinner" /></p>
      <p v-else-if="tagsStore.tags.length === 0" class="tag-state">{{ t['tags.empty'] }}</p>
      <div v-else class="tag-rows">
        <article v-for="tag in tagsStore.tags" :key="tag.id" class="tag-row">
          <span class="tag-swatch" :style="{ background: tag.color }" aria-hidden="true" />
          <div class="tag-main">
            <div class="tag-heading">
              <strong>{{ tag.label }}</strong>
              <span v-if="!tag.enabled" class="tag-off">{{ t['tags.enabled'] }}</span>
            </div>
            <div class="tag-keywords">
              <span v-for="keyword in tag.keywords.slice(0, 5)" :key="keyword" class="keyword">
                {{ keyword }}
              </span>
              <span v-if="tag.keywords.length > 5" class="keyword">
                +{{ tag.keywords.length - 5 }}
              </span>
            </div>
            <div class="tag-fields">
              {{ tag.fields.map(fieldLabel).join(' · ') }}
            </div>
          </div>
          <div class="tag-actions" :aria-label="t['tags.actions']">
            <button
              type="button"
              class="icon-btn"
              :title="t['tags.edit']"
              :aria-label="t['tags.edit']"
              @click="openEdit(tag)"
            >
              <i class="i-ri:edit-2-line" aria-hidden="true" />
            </button>
            <button
              type="button"
              class="icon-btn tag-delete"
              :title="t['tags.delete']"
              :aria-label="t['tags.delete']"
              @click="deleteTarget = tag"
            >
              <i class="i-ri:delete-bin-5-line" aria-hidden="true" />
            </button>
          </div>
        </article>
      </div>
    </section>

    <TagEditorDialog v-model:open="editorOpen" :tag="editing" />
    <TagSuggestionDialog v-model:open="aiOpen" />
    <ConfirmDialog
      :open="!!deleteTarget"
      :title="t['tags.delete']"
      :description="
        deleteTarget ? tf('tags.deleteConfirm', { label: deleteTarget.label }) : undefined
      "
      :confirm-label="t['tags.delete']"
      danger
      :busy="deleting"
      @update:open="(open) => (!open ? (deleteTarget = null) : undefined)"
      @confirm="confirmDelete"
    />
  </div>
</template>

<style scoped>
.tag-subtitle {
  margin: 4px 0 0;
  color: var(--color-text-3);
  font-size: 12px;
}
.head-actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 8px;
}
.tag-state {
  display: flex;
  justify-content: center;
  padding: 34px 16px;
  color: var(--color-text-3);
  font-size: 13px;
}
.tag-rows {
  display: flex;
  flex-direction: column;
}
.tag-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.tag-row:last-child {
  border-bottom: none;
}
.tag-swatch {
  width: 11px;
  height: 11px;
  flex: none;
  border-radius: 999px;
  box-shadow: 0 0 0 3px color-mix(in srgb, currentColor 8%, transparent);
}
.tag-main {
  flex: 1;
  min-width: 0;
}
.tag-heading {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.tag-off {
  color: var(--color-text-3);
  font-size: 10px;
}
.tag-keywords {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 5px;
}
.keyword {
  padding: 1px 6px;
  border-radius: 4px;
  color: var(--color-text-2);
  background: var(--n-action-color);
  font-size: 10px;
}
.tag-fields {
  margin-top: 5px;
  color: var(--color-text-3);
  font-size: 10px;
}
.tag-actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
}
.tag-delete {
  color: var(--color-danger);
}
</style>
