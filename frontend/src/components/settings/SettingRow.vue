<script setup lang="ts">
// 一行设置：左侧名称和说明，右侧控件；stack 时控件放在下方。900px 以下改为上下排列（compact 的开关仍在右侧）
withDefaults(defineProps<{
  title?: string
  description?: string
  /** 关联控件的 id，点击名称时聚焦 */
  for?: string
  /** 控件放在说明下方，适合选择卡片、文本框 */
  stack?: boolean
  /** 暂不生效：名称和控件变淡，不能操作 */
  disabled?: boolean
  /** 有未保存的更改 */
  dirty?: boolean
  /** 控件很窄（如开关），说明文字占满剩余宽度 */
  compact?: boolean
}>(), { stack: false, disabled: false, dirty: false, compact: false })
</script>

<template>
  <div
    :class="[
      'grid gap-2.5 px-4 py-3.5 min-[900px]:px-[18px] min-[900px]:py-4',
      stack ? 'grid-cols-1'
      : compact ? 'grid-cols-[minmax(0,1fr)_auto] items-center gap-4'
      : 'grid-cols-1 min-[900px]:grid-cols-[minmax(0,1fr)_minmax(200px,320px)] min-[900px]:items-center min-[900px]:gap-6',
    ]"
  >
    <div v-if="title || $slots.title">
      <component :is="$props.for ? 'label' : 'div'" :for="$props.for" :class="['flex items-center gap-1.5 text-[13.5px] font-[560] text-oi-ink', disabled && 'opacity-45']">
        <slot name="title">{{ title }}</slot>
        <span v-if="dirty" class="h-1.5 w-1.5 flex-none rounded-full bg-oi-warn" title="未保存" />
        <span v-if="dirty" class="sr-only">（未保存）</span>
      </component>
      <div v-if="description || $slots.description" class="mt-0.5 text-[12.5px] leading-normal text-oi-muted">
        <slot name="description">{{ description }}</slot>
      </div>
    </div>
    <div
      v-if="$slots.default"
      :class="[stack ? 'min-w-0' : 'flex min-w-0 min-[900px]:justify-end', disabled && 'pointer-events-none opacity-45']"
      :inert="disabled || undefined"
    >
      <slot />
    </div>
  </div>
</template>
