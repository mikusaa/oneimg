<template>
  <div class="w-full">
    <button
      ref="triggerRef"
      type="button"
      class="tag-selector-trigger"
      :class="{ 'cursor-not-allowed opacity-60': disabled }"
      :disabled="disabled"
      :aria-expanded="open"
      aria-haspopup="dialog"
      @click="toggle"
    >
      <span v-if="selectedOptions.length === 0" class="truncate text-slate-500 dark:text-slate-400">
        {{ emptyLabel }}
      </span>
      <span v-else class="flex min-w-0 flex-1 items-center gap-1.5 overflow-hidden">
        <span
          v-for="option in summaryOptions"
          :key="String(option.value)"
          class="max-w-[120px] shrink truncate rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-700 dark:bg-white/10 dark:text-slate-200"
        >
          {{ option.label }}
        </span>
        <span
          v-if="remainingCount > 0"
          class="shrink-0 rounded-md bg-slate-900 px-2 py-1 text-xs text-white dark:bg-white dark:text-slate-900"
        >
          +{{ remainingCount }}
        </span>
      </span>
      <i class="ri-arrow-down-s-line shrink-0 text-lg text-slate-400 transition" :class="{ 'rotate-180': open }"></i>
    </button>

    <Teleport to="body">
      <Transition name="tag-scrim">
        <div
          v-if="open && isMobile"
          class="app-scrim fixed inset-0 z-[11990] bg-slate-950/50 backdrop-blur-sm"
          @mousedown.self="close"
        ></div>
      </Transition>
      <Transition name="tag-panel">
        <section
          v-if="open"
          ref="panelRef"
          class="tag-selector-panel app-material"
          :class="isMobile ? 'tag-selector-panel-mobile' : 'tag-selector-panel-desktop'"
          :style="panelStyle"
          role="dialog"
          aria-label="选择标签"
          @mousedown.stop
        >
        <div class="border-b border-slate-200 p-2.5 dark:border-white/10">
          <div class="relative">
            <i class="ri-search-line absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"></i>
            <input
              ref="searchRef"
              v-model="search"
              type="search"
              class="input-modern min-h-10 py-2 pl-9 pr-11"
              placeholder="搜索标签"
              :maxlength="maxLength"
              @keydown.enter.prevent="handleEnter"
              @keydown.down.prevent="focusOption(0)"
              @keydown.up.prevent="focusOption(-1)"
            />
            <button
              v-if="search"
              type="button"
              class="pressable absolute right-0 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-lg text-slate-400 hover:text-slate-700 dark:hover:text-white"
              title="清空搜索"
              aria-label="清空标签搜索"
              @click="search = ''"
            >
              <i class="ri-close-line"></i>
            </button>
          </div>
        </div>

        <div class="min-h-0 max-h-64 flex-1 overflow-y-auto p-1.5 sm:max-h-72">
          <button
            v-for="option in filteredOptions"
            :key="String(option.value)"
            type="button"
            class="pressable flex min-h-10 w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm"
            :class="isSelected(option.value)
              ? 'bg-slate-100 text-slate-900 dark:bg-white/10 dark:text-white'
              : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-white/5 dark:hover:text-white'"
            :disabled="option.disabled"
            data-tag-option
            @click="selectOption(option)"
            @keydown.down.prevent="moveOptionFocus(1)"
            @keydown.up.prevent="moveOptionFocus(-1)"
          >
            <span
              class="flex h-5 w-5 shrink-0 items-center justify-center border text-xs"
              :class="[
                multiple ? 'rounded' : 'rounded-full',
                isSelected(option.value)
                  ? 'border-slate-900 bg-slate-900 text-white dark:border-white dark:bg-white dark:text-slate-900'
                  : 'border-slate-300 bg-white text-transparent dark:border-slate-600 dark:bg-slate-900'
              ]"
            >
              <i :class="multiple ? 'ri-check-line' : 'ri-circle-fill'" class="text-[11px]"></i>
            </span>
            <span class="min-w-0 flex-1 truncate">{{ option.label }}</span>
          </button>

          <div v-if="filteredOptions.length === 0 && !canCreate" class="px-3 py-8 text-center text-sm text-slate-400">
            没有匹配的标签
          </div>
        </div>

        <div v-if="canCreate || createError" class="border-t border-slate-200 p-2.5 dark:border-white/10">
          <button
            v-if="canCreate"
            type="button"
            class="flex min-h-10 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm text-slate-700 transition hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-white/10"
            :disabled="creating"
            @click="createTag"
          >
            <i :class="creating ? 'ri-loader-4-line animate-spin' : 'ri-add-line'"></i>
            <span class="min-w-0 truncate">创建“{{ normalizedSearch }}”</span>
          </button>
          <p v-if="createError" class="mt-1.5 px-2 text-xs text-red-500">{{ createError }}</p>
        </div>

        <footer v-if="multiple" class="flex items-center justify-between gap-2 border-t border-slate-200 p-2.5 dark:border-white/10">
          <button type="button" class="soft-button min-h-9 px-3 py-1.5" :disabled="draftValues.length === 0" @click="clearSelection">
            清空
          </button>
          <div class="flex items-center gap-2">
            <span class="text-xs text-slate-400">已选 {{ draftValues.length }} 个</span>
            <button v-if="confirm" type="button" class="primary-button min-h-9 px-3 py-1.5" @click="applySelection">
              应用
            </button>
            <button v-else type="button" class="primary-button min-h-9 px-3 py-1.5" @click="close">
              完成
            </button>
          </div>
        </footer>
        </section>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { lockBodyScroll, unlockBodyScroll } from '@/utils/scrollLock.js'

const props = defineProps({
  modelValue: { type: [Array, String, Number], default: null },
  options: { type: Array, default: () => [] },
  multiple: { type: Boolean, default: false },
  confirm: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
  emptyLabel: { type: String, default: '选择标签' },
  allowCreate: { type: Boolean, default: false },
  createOption: { type: Function, default: null },
  maxLength: { type: Number, default: 10 }
})

const emit = defineEmits(['update:modelValue'])
const triggerRef = ref(null)
const panelRef = ref(null)
const searchRef = ref(null)
const open = ref(false)
const search = ref('')
const draft = ref(props.multiple ? [] : null)
const isMobile = ref(false)
const panelStyle = ref({})
const creating = ref(false)
const createError = ref('')
let ownsScrollLock = false

const valuesEqual = (left, right) => String(left) === String(right)
const normalizeValues = value => props.multiple
  ? (Array.isArray(value) ? [...value] : [])
  : (value ?? null)

const selectedOptions = computed(() => {
  const values = props.multiple
    ? (Array.isArray(props.modelValue) ? props.modelValue : [])
    : (props.modelValue === null || props.modelValue === undefined || props.modelValue === '' ? [] : [props.modelValue])
  return values
    .map(value => props.options.find(option => valuesEqual(option.value, value)))
    .filter(Boolean)
})
const summaryOptions = computed(() => selectedOptions.value.slice(0, 2))
const remainingCount = computed(() => Math.max(0, selectedOptions.value.length - summaryOptions.value.length))
const normalizedSearch = computed(() => search.value.trim())
const filteredOptions = computed(() => {
  const keyword = normalizedSearch.value.toLocaleLowerCase()
  if (!keyword) return props.options
  return props.options.filter(option => String(option.label).toLocaleLowerCase().includes(keyword))
})
const exactMatch = computed(() => props.options.find(option => (
  String(option.label).toLocaleLowerCase() === normalizedSearch.value.toLocaleLowerCase()
)))
const canCreate = computed(() => Boolean(
  props.allowCreate
  && props.createOption
  && normalizedSearch.value
  && normalizedSearch.value.length <= props.maxLength
  && !exactMatch.value
))
const draftValues = computed(() => Array.isArray(draft.value) ? draft.value : [])

const isSelected = value => props.multiple
  ? draftValues.value.some(item => valuesEqual(item, value))
  : valuesEqual(draft.value, value)

const commit = value => emit('update:modelValue', normalizeValues(value))

const selectOption = option => {
  if (option.disabled) return
  createError.value = ''
  if (!props.multiple) {
    draft.value = option.value
    commit(option.value)
    close()
    return
  }

  draft.value = isSelected(option.value)
    ? draftValues.value.filter(value => !valuesEqual(value, option.value))
    : [...draftValues.value, option.value]
  if (!props.confirm) commit(draft.value)
}

const clearSelection = () => {
  draft.value = props.multiple ? [] : null
  if (!props.confirm) commit(draft.value)
}

const applySelection = () => {
  commit(draft.value)
  close()
}

const createTag = async () => {
  if (!canCreate.value || creating.value) return
  creating.value = true
  createError.value = ''
  try {
    const option = await props.createOption(normalizedSearch.value)
    if (option && option.value !== undefined) {
      selectOption(option)
      search.value = ''
    }
  } catch (error) {
    createError.value = error?.message || '创建标签失败'
  } finally {
    creating.value = false
  }
}

const handleEnter = () => {
  if (canCreate.value) {
    createTag()
  } else if (filteredOptions.value.length === 1) {
    selectOption(filteredOptions.value[0])
  }
}

const optionElements = () => [...(panelRef.value?.querySelectorAll('[data-tag-option]:not(:disabled)') || [])]

const focusOption = index => {
  const options = optionElements()
  if (options.length === 0) return
  const target = index < 0 ? options.length - 1 : Math.min(index, options.length - 1)
  options[target]?.focus()
}

const moveOptionFocus = direction => {
  const options = optionElements()
  const currentIndex = options.indexOf(document.activeElement)
  if (currentIndex < 0) return focusOption(direction > 0 ? 0 : -1)
  options[(currentIndex + direction + options.length) % options.length]?.focus()
}

const positionPanel = async () => {
  if (!open.value || !triggerRef.value) return
  isMobile.value = window.innerWidth < 640
  if (isMobile.value && !ownsScrollLock) {
    lockBodyScroll()
    ownsScrollLock = true
  } else if (!isMobile.value && ownsScrollLock) {
    unlockBodyScroll()
    ownsScrollLock = false
  }
  if (isMobile.value) {
    panelStyle.value = {}
    return
  }

  await nextTick()
  const triggerRect = triggerRef.value.getBoundingClientRect()
  const panelWidth = Math.min(Math.max(triggerRect.width, 300), window.innerWidth - 24)
  const panelHeight = panelRef.value?.offsetHeight || 380
  const left = Math.min(Math.max(12, triggerRect.left), window.innerWidth - panelWidth - 12)
  const fitsBelow = triggerRect.bottom + 8 + panelHeight <= window.innerHeight - 12
  const top = fitsBelow
    ? triggerRect.bottom + 8
    : Math.max(12, triggerRect.top - panelHeight - 8)
  panelStyle.value = { width: `${panelWidth}px`, left: `${left}px`, top: `${top}px` }
}

const show = async () => {
  if (props.disabled) return
  draft.value = normalizeValues(props.modelValue)
  search.value = ''
  createError.value = ''
  open.value = true
  await nextTick()
  await positionPanel()
  searchRef.value?.focus()
}

const close = () => {
  if (!open.value) return
  open.value = false
  search.value = ''
  createError.value = ''
  if (ownsScrollLock) {
    unlockBodyScroll()
    ownsScrollLock = false
  }
  nextTick(() => triggerRef.value?.focus())
}

const toggle = () => open.value ? close() : show()

const handlePointerDown = event => {
  if (!open.value) return
  if (triggerRef.value?.contains(event.target) || panelRef.value?.contains(event.target)) return
  close()
}

const handleKeydown = event => {
  if (open.value && event.key === 'Escape') close()
}

watch(() => props.modelValue, value => {
  if (!open.value) draft.value = normalizeValues(value)
}, { deep: true })

onMounted(() => {
  document.addEventListener('pointerdown', handlePointerDown)
  document.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', positionPanel)
  window.addEventListener('scroll', positionPanel, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', handlePointerDown)
  document.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', positionPanel)
  window.removeEventListener('scroll', positionPanel, true)
  if (ownsScrollLock) unlockBodyScroll()
})
</script>

<style scoped>
.tag-selector-trigger {
  @apply flex min-h-11 w-full min-w-0 items-center justify-between gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-left text-sm outline-none hover:border-slate-300 focus:border-primary/70 focus:ring-4 focus:ring-primary/10 dark:border-white/10 dark:bg-slate-900 dark:hover:border-white/20 dark:focus:border-primary/70 dark:focus:ring-primary/15;
  transition: transform var(--ui-duration-fast) var(--ui-ease), border-color var(--ui-duration-fast) var(--ui-ease), box-shadow var(--ui-duration-fast) var(--ui-ease);
}

.tag-selector-panel {
  @apply z-[12000] flex max-h-[calc(100vh-24px)] origin-top flex-col overflow-hidden rounded-lg border border-slate-200 bg-white shadow-2xl dark:border-white/10 dark:bg-slate-900;
}

.tag-selector-panel-desktop {
  @apply fixed;
}

.tag-selector-panel-mobile {
  @apply fixed left-3 right-3 top-1/2 -translate-y-1/2;
}

.tag-scrim-enter-active,
.tag-scrim-leave-active,
.tag-panel-enter-active,
.tag-panel-leave-active {
  transition: opacity 160ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.tag-scrim-enter-from,
.tag-scrim-leave-to,
.tag-panel-enter-from,
.tag-panel-leave-to {
  opacity: 0;
}

.tag-panel-desktop.tag-panel-enter-from,
.tag-panel-desktop.tag-panel-leave-to {
  transform: translateY(-4px) scale(0.985);
}

.tag-panel-mobile.tag-panel-enter-from,
.tag-panel-mobile.tag-panel-leave-to {
  transform: translateY(calc(-50% + 8px)) scale(0.985);
}

@media (prefers-reduced-motion: reduce) {
  .tag-panel-desktop.tag-panel-enter-from,
  .tag-panel-desktop.tag-panel-leave-to {
    transform: none;
  }

  .tag-panel-mobile.tag-panel-enter-from,
  .tag-panel-mobile.tag-panel-leave-to {
    transform: translateY(-50%);
  }
}
</style>
