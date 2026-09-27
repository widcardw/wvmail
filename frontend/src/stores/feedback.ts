import { defineStore } from 'pinia'
import { createToaster } from '@ark-ui/vue/toast'

export type ToastKind = 'info' | 'success' | 'error' | 'warn'

/**
 * One toaster for the whole app: Ark owns the queue, the stacking, the auto
 * dismiss timer and the hotkey, so this store only maps our four kinds onto it and
 * components/ToastHost.vue only renders the parts. Call sites keep using
 * feedback.success/error/info/warn.
 */
export const toaster = createToaster({
  placement: 'top-end',
  overlap: false,
  gap: 8,
  duration: 3000,
})

export const useFeedbackStore = defineStore('feedback', {
  actions: {
    success(text: string) {
      toaster.create({ title: text, type: 'success' })
    },
    error(text: string) {
      toaster.create({ title: text, type: 'error' })
    },
    info(text: string) {
      toaster.create({ title: text, type: 'info' })
    },
    warn(text: string) {
      toaster.create({ title: text, type: 'warning' })
    },
  },
})
