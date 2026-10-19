<script setup lang="ts">
// 设置分组：标题、说明和带边框的设置列表（账户设置、系统设置共用）
defineProps<{
  title: string
  description?: string
  /** 锚点 id，侧边子导航用它滚动定位 */
  id?: string
}>()
</script>

<template>
  <section :id="id" class="mb-7 scroll-mt-5" :aria-labelledby="id ? `${id}-title` : undefined">
    <h2 :id="id ? `${id}-title` : undefined" class="mb-1 flex items-center gap-2.5 text-[15px] font-[640] text-oi-ink">
      {{ title }}<slot name="badge" />
      <span v-if="$slots.actions" class="ml-auto flex items-center gap-2"><slot name="actions" /></span>
    </h2>
    <p v-if="description || $slots.description" class="mb-3 text-[13px] text-oi-muted">
      <slot name="description">{{ description }}</slot>
    </p>
    <slot name="before" />
    <div class="divide-y divide-oi-line rounded-oi-lg border border-oi-line bg-oi-raised">
      <slot />
    </div>
  </section>
</template>
