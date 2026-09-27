<script setup lang="ts">
import { computed, ref } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import { Field } from '@ark-ui/vue/field'
import { Tabs } from '@ark-ui/vue/tabs'
import { useI18n } from '~/i18n'
import ReminderTimeline from '~/components/reminder/ReminderTimeline.vue'
import type { Reminder } from '~/types/reminder'
import {
  addDays,
  endOfMonth,
  epochSeconds,
  startOfDay,
  startOfMonth,
  startOfWeek,
} from '~/composables/reminder-date'

const { t } = useI18n()

const now = new Date()
const todayStart = startOfDay(now)
const todayEnd = addDays(todayStart, 1)
const weekStart = startOfWeek(now)
const weekEnd = addDays(weekStart, 7)
const monthStart = startOfMonth(now)
const monthEnd = endOfMonth(now)

// Local placeholder data. TODO: replace with a ReminderService store once the
// backend provides one — the Tabs + timeline UI below already consumes a plain
// `Reminder[]`.
function pickDateInMonthOutsideWeek(): Date {
  const last = addDays(monthEnd, -1)
  for (let d = last; d >= monthStart; d = addDays(d, -1)) {
    if (d < weekStart || d >= weekEnd) return d
  }
  return last
}

const monthSpot = pickDateInMonthOutsideWeek()

const reminders = ref<Reminder[]>([
  { id: crypto.randomUUID(), title: '晨会 · 团队同步', time: epochSeconds(now, 9, 30), done: true },
  { id: crypto.randomUUID(), title: '设计评审', time: epochSeconds(now, 14, 0), done: false },
  { id: crypto.randomUUID(), title: '健身', time: epochSeconds(now, 19, 30), done: false },
  {
    id: crypto.randomUUID(),
    title: '回复客户邮件',
    time: epochSeconds(addDays(now, 1), 11, 0),
    done: false,
  },
  {
    id: crypto.randomUUID(),
    title: '牙医预约',
    time: epochSeconds(addDays(now, 2), 15, 30),
    done: false,
  },
  {
    id: crypto.randomUUID(),
    title: '季度汇报准备',
    time: epochSeconds(monthSpot, 9, 0),
    done: false,
  },
  {
    id: crypto.randomUUID(),
    title: '信用卡还款',
    time: epochSeconds(monthSpot, 18, 0),
    done: false,
  },
])

function inRange(reminder: Reminder, from: Date, to: Date): boolean {
  const time = reminder.time * 1000
  return time >= from.getTime() && time < to.getTime()
}

function filterBy(from: Date, to: Date): Reminder[] {
  return reminders.value.filter((r) => inRange(r, from, to)).sort((a, b) => a.time - b.time)
}

const todayItems = computed(() => filterBy(todayStart, todayEnd))
const weekItems = computed(() => filterBy(weekStart, weekEnd))
const monthItems = computed(() => filterBy(monthStart, monthEnd))

function toggle(id: string) {
  const reminder = reminders.value.find((r) => r.id === id)
  if (reminder) reminder.done = !reminder.done
}

const showAdd = ref(false)
const newTitle = ref('')
const newTime = ref('')

function toLocalInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function openAdd() {
  const next = new Date()
  next.setHours(next.getHours() + 1, 0, 0, 0)
  newTitle.value = ''
  newTime.value = toLocalInput(next)
  showAdd.value = true
}

function submitAdd() {
  const title = newTitle.value.trim()
  if (!title || !newTime.value) return
  const time = Math.floor(new Date(newTime.value).getTime() / 1000)
  if (Number.isNaN(time)) return
  // `filterBy` re-sorts on every render, so the new item drops into time order automatically.
  reminders.value.push({ id: crypto.randomUUID(), title, time, done: false })
  showAdd.value = false
}
</script>

<template>
  <div class="content">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['reminders.title'] }}</h2>
        <button type="button" class="btn btn--sm btn--primary" @click="openAdd">
          <i class="i-ri:add-line" aria-hidden="true" />
          {{ t['reminders.add'] }}
        </button>
      </header>

      <Tabs.Root default-value="today" class="tabs">
        <Tabs.List class="tabs__list">
          <Tabs.Trigger class="tabs__trigger" value="today">
            {{ t['reminders.tab.today'] }}
          </Tabs.Trigger>
          <Tabs.Trigger class="tabs__trigger" value="week">
            {{ t['reminders.tab.week'] }}
          </Tabs.Trigger>
          <Tabs.Trigger class="tabs__trigger" value="month">
            {{ t['reminders.tab.month'] }}
          </Tabs.Trigger>
          <Tabs.Indicator class="tabs__indicator" />
        </Tabs.List>

        <Tabs.Content class="tabs__content" value="today">
          <ReminderTimeline :reminders="todayItems" @toggle="toggle" />
        </Tabs.Content>
        <Tabs.Content class="tabs__content" value="week">
          <ReminderTimeline :reminders="weekItems" @toggle="toggle" />
        </Tabs.Content>
        <Tabs.Content class="tabs__content" value="month">
          <ReminderTimeline :reminders="monthItems" @toggle="toggle" />
        </Tabs.Content>
      </Tabs.Root>
    </section>

    <Dialog.Root
      v-if="showAdd"
      :open="showAdd"
      @open-change="(details: unknown) => (showAdd = !!(details as { open?: boolean })?.open)"
    >
      <Dialog.Backdrop class="dlg-back" />
      <Dialog.Positioner class="dlg-pos">
        <Dialog.Content class="dlg-card dlg-card--narrow">
          <h3 class="dlg-title">{{ t['reminders.new'] }}</h3>
          <Field.Root class="d-field">
            <Field.Label class="d-label">{{ t['reminders.field.title'] }}</Field.Label>
            <Field.Input
              v-model="newTitle"
              class="input d-input"
              type="text"
              :placeholder="t['reminders.placeholder.title']"
              @keydown.enter="submitAdd"
            />
          </Field.Root>
          <Field.Root class="d-field">
            <Field.Label class="d-label">{{ t['reminders.field.time'] }}</Field.Label>
            <Field.Input v-model="newTime" class="input d-input" type="datetime-local" />
          </Field.Root>
          <div class="dlg-actions">
            <button type="button" class="btn" @click="showAdd = false">
              {{ t['cancel'] }}
            </button>
            <button
              type="button"
              class="btn btn--primary"
              :disabled="!newTitle.trim()"
              @click="submitAdd"
            >
              {{ t['reminders.submit'] }}
            </button>
          </div>
        </Dialog.Content>
      </Dialog.Positioner>
    </Dialog.Root>
  </div>
</template>

<style scoped>
.content {
  height: 100%;
  padding: 16px;
  overflow-y: auto;
  color: var(--n-text-color);
}
.surface {
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  padding: 0;
  max-width: 720px;
}
.surface__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.surface__head h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}

.tabs__list {
  position: relative;
  display: flex;
  gap: 2px;
  padding: 4px 8px;
  border-bottom: 1px solid var(--n-border-color);
}
.tabs__trigger {
  position: relative;
  z-index: 1;
  padding: 6px 12px;
  border: none;
  background: transparent;
  color: var(--color-text-3);
  font-size: 13px;
  cursor: pointer;
  border-radius: var(--radius-sm) var(--radius-sm) 0 0;
}
.tabs__trigger:hover {
  color: var(--n-text-color);
}
.tabs__trigger[data-selected] {
  color: var(--n-primary-color);
  font-weight: 600;
}
.tabs__indicator {
  position: absolute;
  bottom: -1px;
  height: 2px;
  background: var(--n-primary-color);
  transition:
    left 0.2s ease,
    width 0.2s ease;
}
.tabs__content {
  outline: none;
}

.dlg-title {
  padding-bottom: 0.5rem;
  margin-bottom: 0.5rem;
  border-bottom: 1px solid var(--n-border-color);
}
.d-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}
.d-label {
  font-size: 12px;
  color: var(--color-text-2);
}
.d-input {
  width: 100%;
}
</style>

<style>
/* The global `[data-part='content']` base (src/styles/global.css) wraps every Ark content
 * in a bordered card; `.surface` is already the card here, so strip it for Tabs.Content. */
.tabs [data-scope='tabs'][data-part='content'] {
  border: 0;
  border-radius: 0;
  background: transparent;
}
</style>
