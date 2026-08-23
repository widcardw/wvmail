<script setup lang="ts">
import { NCard, NSelect, NButton, NColorPicker } from 'naive-ui'
import { NForm, NFormItem, SelectOption } from 'naive-ui'
import { useI18n } from '~/i18n'
import { useThemeStore } from '~/stores/theme'
import { computed, Ref } from 'vue'
import { ConfigService, OsService } from '#/wvmail/services'

const { t, locale, setLocale } = useI18n()
const theme = useThemeStore()

const themeOptions: Ref<SelectOption[]> = computed(() => [
  { label: t.value['settings.common.theme.system'], value: 'System' },
  { label: t.value['settings.common.theme.light'], value: 'Light' },
  { label: t.value['settings.common.theme.dark'], value: 'Dark' },
])

const localeOptions: SelectOption[] = [
  { label: 'English', value: 'en-US' },
  { label: '中文', value: 'zh-CN' },
]

function handleLocaleChange(value: string) {
  if (value === 'zh-CN' || value === 'en-US') {
    setLocale(value)
  }
}

async function openConfigFolder() {
  try {
    const dir = await ConfigService.GetConfigDir()
    await OsService.OpenFolder(dir)
  } catch (e) {
    console.error('Failed to open config folder', e)
  }
}
</script>

<template>
  <div style="padding: 16px; max-width: 720px">
    <NCard :title="t['settings.common']" size="small">
      <NForm label-placement="left" label-width="auto">
        <NFormItem label="Language / 语言">
          <NSelect
            :value="locale"
            :options="localeOptions"
            style="width: 200px"
            @update:value="handleLocaleChange"
          />
        </NFormItem>
        <NFormItem :label="t['settings.common.theme']">
          <NSelect
            :value="theme.mode"
            :options="themeOptions"
            style="width: 200px"
            @update:value="(v) => theme.switchTo(v)"
          />
        </NFormItem>
        <NFormItem :label="t['settings.common.primaryColor']">
          <div class="primary-color-picker">
            <NColorPicker
              :value="theme.primaryColor"
              :modes="['hex']"
              :show-alpha="false"
              :swatches="['#18a058', '#2080f0', '#f0a020', '#d03050', '#8a2be2', '#e11d48']"
              @update:value="theme.setPrimaryColor"
            />
          </div>
        </NFormItem>
      </NForm>
      <NButton @click="openConfigFolder">
        {{ t['settings.common.openConfig'] }}
      </NButton>
    </NCard>
  </div>
</template>

<style scoped>
.primary-color-picker {
  width: 200px;
  flex: 0 0 200px;
}
</style>
