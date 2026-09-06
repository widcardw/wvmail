import { defineStore } from 'pinia'

export type ToastKind = 'info' | 'success' | 'error' | 'warn'
export interface ToastItem {
  id: number
  kind: ToastKind
  text: string
}

let seq = 0

/**
 * Minimal transient feedback (success/error/info) for the headless UI; replaces the
 * old library message hook. Render host = components/ToastHost.vue mounted in App.
 */
export const useFeedbackStore = defineStore('feedback', {
  state: () => ({ toasts: [] as ToastItem[] }),
  actions: {
    push(kind: ToastKind, text: string) {
      const id = ++seq
      this.toasts.push({ id, kind, text })
      setTimeout(() => this.dismiss(id), 3000)
    },
    success(text: string) {
      this.push('success', text)
    },
    error(text: string) {
      this.push('error', text)
    },
    info(text: string) {
      this.push('info', text)
    },
    warn(text: string) {
      this.push('warn', text)
    },
    dismiss(id: number) {
      this.toasts = this.toasts.filter((t) => t.id !== id)
    },
  },
})
