// Date-range helpers for the reminders timeline (local time).
// Week starts on Monday.

export function startOfDay(d: Date): Date {
  const x = new Date(d)
  x.setHours(0, 0, 0, 0)
  return x
}

export function addDays(d: Date, n: number): Date {
  const x = new Date(d)
  x.setDate(x.getDate() + n)
  return x
}

export function startOfWeek(d: Date): Date {
  const x = startOfDay(d)
  const dow = (x.getDay() + 6) % 7 // 0 = Monday
  return addDays(x, -dow)
}

export function startOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth(), 1)
}

export function endOfMonth(d: Date): Date {
  return new Date(d.getFullYear(), d.getMonth() + 1, 1)
}

/** Local-time wall clock → epoch seconds. */
export function epochSeconds(date: Date, hour = 0, minute = 0): number {
  return Math.floor(
    new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour, minute).getTime() / 1000,
  )
}

/** `datetime-local` wants a local, seconds-less value. */
export function toLocalInputValue(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
