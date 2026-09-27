<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useVirtualList } from '@vueuse/core'
import { ConfigService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import UiCheckbox from '~/components/UiCheckbox.vue'
import type { LLMConfig } from '~/types/config'

const props = defineProps<{
  /** Config used for the catalog request (provider + base URL + key). */
  config: LLMConfig
  /** Already picked models; edited here and handed back on confirm. */
  selected: string[]
}>()

const emit = defineEmits<{
  confirm: [models: string[]]
  cancel: []
}>()

const { t } = useI18n()

const loading = ref(true)
const error = ref('')
const available = ref<string[]>([])
const picked = ref<string[]>([...props.selected])
const query = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    // Resolves to the model ids; rejects with the provider's reason (which the
    // user needs, because a provider without a catalog has to be typed by hand).
    available.value = await ConfigService.ListLLMModels({ ...props.config })
  } catch (e) {
    available.value = []
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}
onMounted(load)

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  return q ? available.value.filter((model) => model.toLowerCase().includes(q)) : available.value
})

// Rows have a fixed height, so only the visible slice is rendered — gateways can
// return well over a thousand models and a plain v-for would re-render all of
// them on every keystroke.
const { list, containerProps, wrapperProps } = useVirtualList(filtered, {
  itemHeight: 30,
  overscan: 8,
})

function isPicked(model: string) {
  return picked.value.includes(model)
}

function toggle(model: string) {
  picked.value = isPicked(model)
    ? picked.value.filter((m) => m !== model)
    : [...picked.value, model]
}

function selectAllFiltered() {
  picked.value = [...new Set([...picked.value, ...filtered.value])]
}

function clearAll() {
  picked.value = []
}
</script>

<template>
  <div class="picker">
    <div class="picker__bar">
      <input
        v-model="query"
        class="picker__search"
        type="search"
        :placeholder="t['settings.llm.modelsSearch']"
        :disabled="loading || available.length === 0"
      />
      <button
        type="button"
        class="btn btn--sm"
        :disabled="filtered.length === 0"
        @click="selectAllFiltered()"
      >
        {{ t['settings.llm.modelsSelectAll'] }}
      </button>
      <button type="button" class="btn btn--sm" :disabled="picked.length === 0" @click="clearAll()">
        {{ t['settings.llm.modelsClear'] }}
      </button>
    </div>

    <p v-if="loading" class="picker__state">{{ t['settings.llm.testing'] }}</p>
    <p v-else-if="error" class="picker__state picker__state--err">{{ error }}</p>
    <p v-else-if="available.length === 0" class="picker__state">
      {{ t['settings.llm.modelsHint'] }}
    </p>
    <div v-else v-bind="containerProps" class="picker__scroll">
      <div v-bind="wrapperProps">
        <UiCheckbox
          v-for="item in list"
          :key="item.data"
          class="picker__row"
          label-class="picker__id font-mono"
          :title="item.data"
          :checked="isPicked(item.data)"
          @update:checked="toggle(item.data)"
        >
          {{ item.data }}
        </UiCheckbox>
      </div>
    </div>

    <div class="picker__foot">
      <span class="picker__count">
        {{
          t['settings.llm.modelsSelected']
            .replace('{n}', String(picked.length))
            .replace('{total}', String(available.length))
        }}
      </span>
      <span class="picker__spacer" />
      <button type="button" class="btn btn--sm" @click="emit('cancel')">
        {{ t['settings.llm.cancel'] }}
      </button>
      <button type="button" class="btn btn--sm btn--primary" @click="emit('confirm', picked)">
        {{ t['settings.llm.modelsApply'] }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.picker {
  margin-top: 8px;
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  background: var(--color-surface-2);
}
.picker__bar {
  display: flex;
  gap: 6px;
  padding: 8px;
}
.picker__search {
  flex: 1;
  min-width: 0;
}
.picker__scroll {
  max-height: 240px;
  border-top: 1px solid var(--n-border-color);
}
.picker__row {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 30px;
  padding: 0 10px;
  cursor: pointer;
}
.picker__row:hover {
  background: var(--color-hover);
}
.picker__id {
  overflow: hidden;
  font-size: 12px;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.picker__foot {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px;
  border-top: 1px solid var(--n-border-color);
}
.picker__count {
  font-size: 12px;
  color: var(--color-text-2);
}
.picker__spacer {
  flex: 1;
}
.picker__state {
  margin: 0;
  padding: 10px;
  font-size: 12px;
  border-top: 1px solid var(--n-border-color);
  color: var(--color-text-2);
}
.picker__state--err {
  color: var(--color-danger);
}
</style>
