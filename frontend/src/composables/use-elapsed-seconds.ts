import { onUnmounted, ref, watch } from 'vue'

/**
 * Counts up once per second while `active` is true, resetting to 0 each time it
 * turns true. Used to show how long a connection test has been running: a bare
 * ellipsis hides whether the attempt is progressing or stuck until it times out.
 */
export function useElapsedSeconds(active: () => boolean) {
  const seconds = ref(0)
  let timer: ReturnType<typeof setInterval> | null = null

  watch(
    active,
    (on) => {
      if (timer !== null) {
        clearInterval(timer)
        timer = null
      }
      seconds.value = 0
      if (on) {
        timer = setInterval(() => {
          seconds.value += 1
        }, 1000)
      }
    },
    { immediate: true },
  )

  onUnmounted(() => {
    if (timer !== null) clearInterval(timer)
  })

  return seconds
}
