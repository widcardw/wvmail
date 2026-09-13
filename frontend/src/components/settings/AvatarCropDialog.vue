<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import { ImageCropper, useImageCropper } from '@ark-ui/vue/image-cropper'
import { useI18n } from '~/i18n'
import { useFeedbackStore } from '~/stores/feedback'

const props = defineProps<{
  open: boolean
  imageSrc: string
  imageAspect: number
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  cropped: [image: Blob]
}>()

const { t } = useI18n()
const fb = useFeedbackStore()
const imageCropper = useImageCropper({ aspectRatio: 1 })
const cropSaving = ref(false)

const cropViewportStyle = computed(() => {
  const maxWidth = typeof window === 'undefined' ? 420 : Math.max(220, window.innerWidth - 100)
  const maxHeight = typeof window === 'undefined' ? 360 : Math.min(420, window.innerHeight * 0.58)
  const width = Math.min(420, maxWidth, maxHeight * props.imageAspect)
  return { width: `${width}px`, height: `${width / props.imageAspect}px` }
})

function close() {
  emit('update:open', false)
}

async function confirm() {
  cropSaving.value = true
  try {
    const result = await imageCropper.value.getCroppedImage({ output: 'blob', type: 'image/png' })
    if (!(result instanceof Blob)) {
      fb.error(t.value['settings.accounts.crop_failed'])
      return
    }
    emit('cropped', result)
  } catch (error) {
    console.error('Failed to crop avatar', error)
    fb.error(t.value['settings.accounts.crop_failed'])
  } finally {
    cropSaving.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) imageCropper.value.reset?.()
  },
)

onUnmounted(() => imageCropper.value.reset?.())
</script>

<template>
  <Dialog.Root
    :open="props.open"
    lazy-mount
    unmount-on-exit
    @update:open="emit('update:open', $event)"
  >
    <Teleport to="body">
      <Dialog.Backdrop class="crop-dialog-backdrop" />
      <Dialog.Positioner class="crop-dialog-positioner">
        <Dialog.Content class="crop-dialog-content">
          <header class="crop-dialog__head">
            <div>
              <Dialog.Title class="crop-dialog__title">
                {{ t['settings.accounts.crop_avatar'] }}
              </Dialog.Title>
              <Dialog.Description class="crop-dialog__description">
                {{ t['settings.accounts.crop_avatar_hint'] }}
              </Dialog.Description>
            </div>
            <Dialog.CloseTrigger
              type="button"
              class="crop-dialog__close"
              :aria-label="t['settings.accounts.close_crop']"
              @click="close"
            >
              <i class="i-ri:close-line" aria-hidden="true" />
            </Dialog.CloseTrigger>
          </header>

          <ImageCropper.RootProvider :value="imageCropper" class="avatar-cropper">
            <ImageCropper.Viewport class="avatar-cropper__viewport" :style="cropViewportStyle">
              <ImageCropper.Image
                v-if="props.imageSrc"
                :key="props.imageSrc"
                class="avatar-cropper__image"
                :src="props.imageSrc"
                :alt="t['settings.accounts.crop_image_alt']"
              />
              <ImageCropper.Selection class="avatar-cropper__selection">
                <ImageCropper.Handle
                  v-for="position in ImageCropper.handles"
                  :key="position"
                  class="avatar-cropper__handle"
                  :position="position"
                >
                  <div />
                </ImageCropper.Handle>
                <ImageCropper.Grid class="avatar-cropper__grid" axis="horizontal" />
                <ImageCropper.Grid class="avatar-cropper__grid" axis="vertical" />
              </ImageCropper.Selection>
            </ImageCropper.Viewport>
          </ImageCropper.RootProvider>

          <footer class="crop-dialog__foot">
            <button type="button" class="btn" @click="close">
              {{ t['settings.accounts.cancel'] }}
            </button>
            <button type="button" class="btn btn--primary" :disabled="cropSaving" @click="confirm">
              {{ cropSaving ? '…' : t['settings.accounts.crop_avatar_done'] }}
            </button>
          </footer>
        </Dialog.Content>
      </Dialog.Positioner>
    </Teleport>
  </Dialog.Root>
</template>

<style scoped>
.crop-dialog-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1100;
  background: rgb(0 0 0 / 55%);
}
.crop-dialog-positioner {
  position: fixed;
  inset: 0;
  z-index: 1101;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}
.crop-dialog-content {
  position: relative;
  display: flex;
  flex-direction: column;
  width: min(520px, calc(100vw - 32px));
  max-height: calc(100vh - 32px);
  overflow: auto;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 20px 50px rgb(0 0 0 / 30%);
}
.crop-dialog__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px 12px;
}
.crop-dialog__title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.crop-dialog__description {
  margin-top: 5px;
  color: var(--color-text-2);
  font-size: 12px;
}
.crop-dialog__close {
  display: inline-flex;
  flex: 0 0 26px;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  color: var(--color-text-2);
  cursor: pointer;
  background: transparent;
  border: none;
  border-radius: 4px;
}
.crop-dialog__close:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.crop-dialog__close:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.avatar-cropper {
  --cropper-accent: var(--n-primary-color);
  --cropper-line-color: rgb(255 255 255 / 50%);
  --cropper-overlay-color: rgb(0 0 0 / 50%);
  --cropper-handler-size: 6px;
  --cropper-handler-width: 3px;
  --cropper-line-width: 2px;

  position: relative;
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 0 18px;
  color: var(--n-text-color);
}
.avatar-cropper__viewport {
  position: relative;
  max-width: 100%;
  margin: 0 auto;
  overflow: hidden;
  background: var(--n-action-color);
  border-radius: 5px;
  touch-action: none;
}
.avatar-cropper__image {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
  transform-origin: center center;
  pointer-events: none;
  user-select: none;
  backface-visibility: hidden;
}
.avatar-cropper__selection {
  box-sizing: content-box;
  border: var(--cropper-line-width) solid var(--cropper-line-color);
  box-shadow: 0 0 0 9999px var(--cropper-overlay-color);
  cursor: move;
  outline: none;
  backface-visibility: hidden;
  touch-action: none;
}
.avatar-cropper__selection:focus-visible {
  border-color: var(--cropper-accent);
}
.avatar-cropper__selection[data-dragging] {
  cursor: grabbing;
  border-color: rgb(255 255 255 / 80%);
}
.avatar-cropper__grid {
  position: absolute;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.2s ease;
}
.avatar-cropper__grid[data-axis='horizontal'] {
  inset: 33.33% 0;
  border-top: 1px solid rgb(255 255 255 / 40%);
  border-bottom: 1px solid rgb(255 255 255 / 40%);
}
.avatar-cropper__grid[data-axis='vertical'] {
  inset: 0 33.33%;
  border-right: 1px solid rgb(255 255 255 / 40%);
  border-left: 1px solid rgb(255 255 255 / 40%);
}
.avatar-cropper__grid[data-dragging],
.avatar-cropper__grid[data-panning] {
  opacity: 1;
}
.avatar-cropper__handle {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  width: calc(var(--cropper-handler-size) + 8px);
  height: calc(var(--cropper-handler-size) + 8px);
  touch-action: none;
}
.avatar-cropper__handle > div {
  width: var(--cropper-handler-size);
  height: var(--cropper-handler-size);
  background: var(--color-surface);
  box-shadow: 0 1px 3px rgb(0 0 0 / 30%);
}
.avatar-cropper__handle[data-disabled] {
  display: none;
}
.avatar-cropper__handle[data-position='top-left'],
.avatar-cropper__handle[data-position='top-right'],
.avatar-cropper__handle[data-position='bottom-right'],
.avatar-cropper__handle[data-position='bottom-left'] {
  cursor: nwse-resize;
}
.avatar-cropper__handle[data-position='top-left'] > div {
  border-top: var(--cropper-handler-width) solid var(--cropper-accent);
  border-left: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='top-right'] {
  cursor: nesw-resize;
}
.avatar-cropper__handle[data-position='top-right'] > div {
  border-top: var(--cropper-handler-width) solid var(--cropper-accent);
  border-right: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='bottom-right'] > div {
  border-right: var(--cropper-handler-width) solid var(--cropper-accent);
  border-bottom: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='bottom-left'] {
  cursor: nesw-resize;
}
.avatar-cropper__handle[data-position='bottom-left'] > div {
  border-bottom: var(--cropper-handler-width) solid var(--cropper-accent);
  border-left: var(--cropper-handler-width) solid var(--cropper-accent);
}
.avatar-cropper__handle[data-position='top'] > div,
.avatar-cropper__handle[data-position='bottom'] > div,
.avatar-cropper__handle[data-position='left'] > div,
.avatar-cropper__handle[data-position='right'] > div {
  width: var(--cropper-handler-size);
  height: var(--cropper-handler-size);
  background: var(--cropper-accent);
  border: none;
  border-radius: 9999px;
  opacity: 0;
}
.avatar-cropper__handle[data-position='top']:hover > div,
.avatar-cropper__handle[data-position='bottom']:hover > div,
.avatar-cropper__handle[data-position='left']:hover > div,
.avatar-cropper__handle[data-position='right']:hover > div {
  opacity: 1;
}
.avatar-cropper__handle[data-position='top'],
.avatar-cropper__handle[data-position='bottom'] {
  cursor: ns-resize;
}
.avatar-cropper__handle[data-position='left'],
.avatar-cropper__handle[data-position='right'] {
  cursor: ew-resize;
}
.crop-dialog__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 14px 18px 16px;
}
</style>
