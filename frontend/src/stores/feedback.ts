import { defineStore } from 'pinia'
import { createToaster } from '@ark-ui/vue/toast'
import { useI18n } from '~/i18n'

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

/**
 * Report a failed fetch the way the whole app does: the reason in the console and a
 * toast, because a silent failure looks exactly like "there was nothing to show".
 * Shared by the accounts and search stores.
 */
export function reportFetchFailure(action: string, e: unknown) {
  const { t } = useI18n()
  console.error(action, e)
  const detail = e instanceof Error ? e.message : String(e)
  toaster.create({ title: `${t.value['mail.refreshFailed']}: ${detail}`, type: 'error' })
}
