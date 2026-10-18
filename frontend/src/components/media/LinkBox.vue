<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { toast } from '@/components/ui'
import { copyMediaText } from '@/utils/media'

const props = withDefaults(defineProps<{
  value: string
  /** 输入框的无障碍名称，也用于复制成功的提示 */
  label?: string
}>(), { label: '链接' })

const input = ref<HTMLInputElement | null>(null)
const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

const copy = async () => {
  try {
    await copyMediaText(props.value)
    toast.success(`已复制${props.label}`)
    copied.value = true
    clearTimeout(timer)
    timer = setTimeout(() => { copied.value = false }, 1400)
  } catch {
    input.value?.select()
    toast.error('复制失败，请手动复制')
  }
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<template>
  <div
    class="flex h-[34px] min-w-0 cursor-text items-center gap-1.5 rounded-[9px] border border-oi-line bg-oi-panel pl-2.5 pr-1 transition-[border-color,box-shadow] focus-within:border-oi-accent focus-within:shadow-[0_0_0_3px_var(--oi-ring)]"
    @click="input?.select()"
  >
    <input
      ref="input"
      :value="value"
      readonly
      :aria-label="label"
      class="h-full min-w-0 flex-1 border-0 bg-transparent font-oi-mono text-xs text-oi-text outline-none"
    >
    <button
      type="button"
      :class="[
        'inline-flex h-[26px] flex-none items-center gap-1 rounded-[7px] border px-2 text-xs font-[550] transition-colors focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring',
        copied ? 'border-transparent bg-oi-accent-soft text-oi-accent-2' : 'border-oi-line-2 bg-oi-raised text-oi-ink hover:border-oi-faint',
      ]"
      @click.stop="copy"
    >
      <i :class="copied ? 'ri-check-line' : 'ri-file-copy-line'" aria-hidden="true" />{{ copied ? '已复制' : '复制' }}
    </button>
  </div>
</template>
