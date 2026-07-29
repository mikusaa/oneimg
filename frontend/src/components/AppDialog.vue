<template>
  <Teleport to="body">
    <Transition name="app-dialog">
      <div
        v-if="modelValue"
        class="app-dialog-layer app-scrim fixed inset-0 z-[11000] flex items-center justify-center bg-slate-950/50 p-3 backdrop-blur-sm sm:p-5"
        role="presentation"
        @mousedown.self="handleBackdrop"
      >
        <section
          ref="dialogRef"
          class="app-dialog-panel app-material flex max-h-[calc(100vh-24px)] w-full flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-2xl dark:border-white/10 dark:bg-slate-900 sm:max-h-[calc(100vh-40px)]"
          :class="widthClass"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          tabindex="-1"
          @mousedown.stop
        >
          <header class="flex items-center justify-between gap-4 border-b border-slate-200 px-4 py-3.5 dark:border-white/10 sm:px-5">
            <h2 :id="titleId" class="min-w-0 truncate text-base font-semibold text-slate-900 dark:text-white">
              {{ title }}
            </h2>
            <button type="button" class="icon-button shrink-0" title="关闭" aria-label="关闭" @click="close">
              <i class="ri-close-line"></i>
            </button>
          </header>

          <div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 sm:px-5">
            <slot />
          </div>

          <footer
            v-if="$slots.footer"
            class="flex flex-wrap items-center justify-end gap-2 border-t border-slate-200 px-4 py-3.5 dark:border-white/10 sm:px-5"
          >
            <slot name="footer" />
          </footer>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { lockBodyScroll, unlockBodyScroll } from '@/utils/scrollLock.js'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, required: true },
  widthClass: { type: String, default: 'max-w-lg' },
  closeOnBackdrop: { type: Boolean, default: true },
  initialFocus: { type: String, default: '' }
})

const emit = defineEmits(['update:modelValue', 'close'])
const titleId = `dialog-${useId()}`
const dialogRef = ref(null)
let previousActiveElement = null
let ownsScrollLock = false

const focusableSelector = [
  'button:not([disabled])',
  '[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])'
].join(',')

const focusInitialElement = async () => {
  await nextTick()
  const dialog = dialogRef.value
  if (!dialog) return
  const preferred = props.initialFocus ? dialog.querySelector(props.initialFocus) : null
  const autofocus = dialog.querySelector('[autofocus]')
  const formControl = dialog.querySelector('input:not([disabled]), select:not([disabled]), textarea:not([disabled])')
  const fallback = dialog.querySelector('button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])')
  ;(preferred || autofocus || formControl || fallback || dialog).focus?.({ preventScroll: true })
}

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const handleBackdrop = () => {
  if (props.closeOnBackdrop) close()
}

const handleKeydown = event => {
  if (!props.modelValue) return
  if (event.key === 'Escape') {
    if (document.querySelector('.tag-selector-panel')) return
    close()
    return
  }
  if (event.key !== 'Tab' || !dialogRef.value) return

  const focusable = [...dialogRef.value.querySelectorAll(focusableSelector)]
    .filter(element => element.offsetParent !== null)
  if (focusable.length === 0) {
    event.preventDefault()
    dialogRef.value.focus?.()
    return
  }

  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(() => props.modelValue, async value => {
  if (value) {
    previousActiveElement = document.activeElement
    if (!ownsScrollLock) {
      lockBodyScroll()
      ownsScrollLock = true
    }
    await focusInitialElement()
    return
  }

  if (ownsScrollLock) {
    unlockBodyScroll()
    ownsScrollLock = false
  }
  await nextTick()
  previousActiveElement?.focus?.({ preventScroll: true })
  previousActiveElement = null
}, { immediate: true })

onMounted(() => document.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  if (ownsScrollLock) unlockBodyScroll()
})
</script>

<style scoped>
.app-dialog-enter-active,
.app-dialog-leave-active {
  transition: opacity 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.app-dialog-enter-active .app-dialog-panel,
.app-dialog-leave-active .app-dialog-panel {
  transition: opacity 180ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 220ms cubic-bezier(0.2, 0.8, 0.2, 1), backdrop-filter 220ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.app-dialog-enter-from,
.app-dialog-leave-to {
  opacity: 0;
}

.app-dialog-enter-from .app-dialog-panel,
.app-dialog-leave-to .app-dialog-panel {
  opacity: 0;
  transform: translateY(8px) scale(0.985);
  backdrop-filter: blur(4px);
}

@media (prefers-reduced-motion: reduce) {
  .app-dialog-enter-from .app-dialog-panel,
  .app-dialog-leave-to .app-dialog-panel {
    transform: none;
  }
}
</style>
