import { onBeforeRouteLeave } from 'vue-router'

type PersistOp = () => void | Promise<void>

interface Entry {
  timer: ReturnType<typeof setTimeout>
  op: PersistOp
}

/**
 * Debounced "persist on change + flush on leaving the route" helper.
 *
 * Use inside a <script setup> for any form-like settings page:
 *
 *   const persist = useAutoPersist()
 *   function onLevel(v: string) { form.level = v; persist.schedule('log', () => saveLogs()) }
 *   function onRetention(n: number) { form.retention = n; persist.schedule('log', () => saveLogs()) }
 *   function onSafety() { form.safety = !form.safety; persist.schedule('safety', () => saveSafety()) }
 *
 * `schedule` is keyed by a `kind`: calling it again for the same kind replaces the
 * pending op (latest state wins). `flush()` (on route leave) immediately runs any
 * op that hasn't been debounced yet, so the last change is never lost.
 */
export function useAutoPersist(defaultDelay = 400) {
  const pending = new Map<string, Entry>()

  function schedule(kind: string, op: PersistOp, delay = defaultDelay) {
    const prev = pending.get(kind)
    if (prev) clearTimeout(prev.timer)
    const timer = setTimeout(() => {
      pending.delete(kind)
      void op()
    }, delay)
    pending.set(kind, { timer, op })
  }

  function flush() {
    for (const [kind, entry] of pending) {
      clearTimeout(entry.timer)
      pending.delete(kind)
      void entry.op()
    }
  }

  onBeforeRouteLeave(flush)
  return { schedule, flush }
}
