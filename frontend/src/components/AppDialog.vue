<template>
  <Teleport to="body">
    <Transition name="app-dialog">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-[11000] flex items-center justify-center bg-slate-950/50 p-3 backdrop-blur-sm sm:p-5"
        role="presentation"
        @mousedown.self="handleBackdrop"
      >
        <section
          class="flex max-h-[calc(100vh-24px)] w-full flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-2xl dark:border-white/10 dark:bg-slate-900 sm:max-h-[calc(100vh-40px)]"
          :class="widthClass"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          @mousedown.stop
        >
          <header class="flex items-center justify-between gap-4 border-b border-slate-200 px-4 py-3.5 dark:border-white/10 sm:px-5">
            <h2 :id="titleId" class="min-w-0 truncate text-base font-semibold text-slate-900 dark:text-white">
              {{ title }}
            </h2>
            <button type="button" class="icon-button shrink-0" title="关闭" @click="close">
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
import { onBeforeUnmount, onMounted, useId } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, required: true },
  widthClass: { type: String, default: 'max-w-lg' },
  closeOnBackdrop: { type: Boolean, default: true }
})

const emit = defineEmits(['update:modelValue', 'close'])
const titleId = `dialog-${useId()}`

const close = () => {
  emit('update:modelValue', false)
  emit('close')
}

const handleBackdrop = () => {
  if (props.closeOnBackdrop) close()
}

const handleKeydown = event => {
  if (!props.modelValue || event.key !== 'Escape') return
  if (document.querySelector('.tag-selector-panel')) return
  close()
}

onMounted(() => document.addEventListener('keydown', handleKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', handleKeydown))
</script>

<style scoped>
.app-dialog-enter-active,
.app-dialog-leave-active {
  transition: opacity 160ms ease;
}

.app-dialog-enter-from,
.app-dialog-leave-to {
  opacity: 0;
}
</style>
