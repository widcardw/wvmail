<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '~/i18n'
import type { Reminder } from '~/types/reminder'
import { startOfDay } from '~/composables/reminder-date'

const props = defineProps<{ reminders: Reminder[] }>()
const emit = defineEmits<{ toggle: [id: string] }>()

const { t, locale } = useI18n()

// Group reminders by calendar day, then by time within each day.
const groups = computed(() => {
  const map = new Map<number, Reminder[]>()
  for (const r of props.reminders) {
    const key = startOfDay(new Date(r.time * 1000)).getTime()
    const arr = map.get(key)
    if (arr) arr.push(r)
    else map.set(key, [r])
  }
  return [...map.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([key, items]) => ({
      key,
      date: new Date(key),
      items: [...items].sort((a, b) => a.time - b.time),
    }))
})

const todayKey = startOfDay(new Date()).getTime()
const tomorrowKey = todayKey + 24 * 60 * 60 * 1000

function dayLabel(date: Date): string {
  const key = startOfDay(date).getTime()
  if (key === todayKey) return t('reminders.day.today')
  if (key === tomorrowKey) return t('reminders.day.tomorrow')
  return date.toLocaleDateString(locale.value, { month: 'short', day: 'numeric' })
}

function weekdayLabel(date: Date): string {
  return date.toLocaleDateString(locale.value, { weekday: 'short' })
}

function timeLabel(ts: number): string {
  return new Date(ts * 1000).toLocaleTimeString(locale.value, {
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div v-if="groups.length === 0" class="tl-empty">
    <i class="i-ri:notification-off-line tl-empty__icon" aria-hidden="true" />
    <p>{{ t('reminders.empty') }}</p>
  </div>
  <ol v-else class="tl">
    <li v-for="group in groups" :key="group.key" class="tl-group">
      <div class="tl-date">
        <span class="tl-date__day">{{ dayLabel(group.date) }}</span>
        <span class="tl-date__weekday">{{ weekdayLabel(group.date) }}</span>
      </div>
      <ul class="tl-items">
        <li
          v-for="reminder in group.items"
          :key="reminder.id"
          class="tl-item"
          :class="{ 'tl-item--done': reminder.done }"
        >
          <button
            type="button"
            class="tl-check"
            :class="{ 'tl-check--on': reminder.done }"
            :aria-label="t('reminders.done')"
            :aria-pressed="reminder.done"
            @click="emit('toggle', reminder.id)"
          >
            <i v-if="reminder.done" class="i-ri:check-line" aria-hidden="true" />
          </button>
          <span class="tl-time">{{ timeLabel(reminder.time) }}</span>
          <span class="tl-title">{{ reminder.title }}</span>
        </li>
      </ul>
    </li>
  </ol>
</template>

<style scoped>
.tl {
  list-style: none;
  margin: 0;
  padding: 8px 16px 16px;
}

.tl-group {
  display: flex;
  gap: 14px;
  padding: 10px 0;
}
.tl-group + .tl-group {
  border-top: 1px solid var(--n-border-color);
}

.tl-date {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 1px;
  width: 74px;
  flex-shrink: 0;
  padding-top: 3px;
}
.tl-date__day {
  font-size: 13px;
  font-weight: 600;
  color: var(--n-text-color);
}
.tl-date__weekday {
  font-size: 11px;
  color: var(--color-text-3);
}

.tl-items {
  list-style: none;
  margin: 0;
  padding: 0;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.tl-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  border-left: 2px solid var(--n-primary-color);
  background: var(--color-surface-2);
}
.tl-item--done {
  border-left-color: var(--n-border-color);
  opacity: 0.6;
}
.tl-item--done .tl-title {
  text-decoration: line-through;
}

.tl-check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  padding: 0;
  flex-shrink: 0;
  border: 1.5px solid var(--n-border-color);
  border-radius: 50%;
  background: transparent;
  color: #fff;
  cursor: pointer;
  font-size: 12px;
}
.tl-check:hover {
  border-color: var(--n-primary-color);
}
.tl-check--on {
  background: var(--n-primary-color);
  border-color: var(--n-primary-color);
}

.tl-time {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  color: var(--color-text-3);
  flex-shrink: 0;
  width: 46px;
}

.tl-title {
  font-size: 13px;
  color: var(--n-text-color);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tl-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 16px;
  color: var(--color-text-3);
  font-size: 13px;
}
.tl-empty__icon {
  font-size: 34px;
  opacity: 0.7;
}
</style>
