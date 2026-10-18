<script setup lang="ts">
import { computed } from 'vue'
import { Primitive } from 'reka-ui'

type Variant = 'default' | 'primary' | 'accent' | 'ghost' | 'danger' | 'danger-solid'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: 'md' | 'sm'
  /** 只有图标的方形按钮，需要同时提供 aria-label */
  icon?: boolean
  as?: string
  asChild?: boolean
  type?: 'button' | 'submit' | 'reset'
}>(), {
  variant: 'default',
  size: 'md',
  icon: false,
  as: 'button',
  asChild: false,
  type: 'button',
})

const variants: Record<Variant, string> = {
  default: 'border-oi-line-2 bg-oi-raised text-oi-ink shadow-oi-sm hover:border-oi-faint',
  primary: 'border-transparent bg-oi-btn text-oi-btn-ink hover:opacity-90',
  accent: 'border-transparent bg-oi-accent text-white hover:opacity-90 dark:text-[#06281a]',
  ghost: 'border-transparent bg-transparent text-oi-text hover:bg-oi-hover hover:text-oi-ink',
  danger: 'border-oi-line-2 bg-oi-raised text-oi-danger shadow-oi-sm hover:border-transparent hover:bg-oi-danger-soft',
  'danger-solid': 'border-transparent bg-oi-danger text-white hover:opacity-90',
}

const classes = computed(() => [
  'inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap border font-[550] transition-[background-color,box-shadow,transform,color,border-color,opacity] duration-150',
  'active:translate-y-px disabled:pointer-events-none disabled:opacity-45 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring [&_i]:text-base',
  props.size === 'sm' ? 'h-[30px] rounded-lg text-[12.5px]' : 'h-9 rounded-[10px] text-[13.5px]',
  props.icon ? (props.size === 'sm' ? 'w-[30px]' : 'w-9') : (props.size === 'sm' ? 'px-2.5' : 'px-3.5'),
  variants[props.variant],
])
</script>

<template>
  <Primitive
    :as="as"
    :as-child="asChild"
    :type="as === 'button' && !asChild ? type : undefined"
    :class="classes"
  >
    <slot />
  </Primitive>
</template>
