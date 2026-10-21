<script setup lang="ts">
import { computed } from 'vue'

// 可切换的格式标签；options 之外已有的值也显示出来，可以移除，避免保存时被悄悄丢掉
const props = defineProps<{
  options: readonly string[]
  /** 组的无障碍名称 */
  label: string
  /** 至少保留一项（只算显示出来的） */
  keepOne?: boolean
  /** 保留但不显示的值，如已不能上传的格式 */
  hidden?: readonly string[]
}>()
const model = defineModel<string[]>({ required: true })
const emit = defineEmits<{ blocked: [] }>()

const sameAs = (value: string) => (item: string) => item.toLowerCase() === value.toLowerCase()
const extras = computed(() => model.value.filter(value => ![...props.options, ...(props.hidden ?? [])].some(sameAs(value))))
const isOn = (value: string) => model.value.some(sameAs(value))
const visibleCount = computed(() => model.value.filter(value => !props.hidden?.some(sameAs(value))).length)
const toggle = (value: string) => {
  if (isOn(value)) {
    if (props.keepOne && visibleCount.value === 1) return emit('blocked')
    model.value = model.value.filter(item => item.toLowerCase() !== value.toLowerCase())
  } else model.value = [...model.value, value]
}
const chip = 'inline-flex h-7 items-center gap-1 rounded-lg border px-2.5 font-oi-mono text-xs transition-colors focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring'
</script>

<template>
  <div class="flex flex-wrap gap-1.5" role="group" :aria-label="label">
    <button
      v-for="option in options"
      :key="option"
      type="button"
      :aria-pressed="isOn(option)"
      :class="[chip, isOn(option) ? 'border-transparent bg-oi-accent-soft text-oi-accent-2' : 'border-oi-line-2 text-oi-muted hover:text-oi-ink']"
      @click="toggle(option)"
    >
      <i v-if="isOn(option)" class="ri-check-line" aria-hidden="true" />{{ option }}
    </button>
    <button
      v-for="extra in extras"
      :key="extra"
      type="button"
      :class="[chip, 'border-transparent bg-oi-accent-soft text-oi-accent-2']"
      :title="`已有的设置「${extra}」，点击移除`"
      :aria-label="`移除 ${extra}`"
      @click="toggle(extra)"
    >
      {{ extra }}<i class="ri-close-line" aria-hidden="true" />
    </button>
  </div>
</template>
