<script setup lang="ts">
import AppBrand from '@/components/shell/AppBrand.vue'

// 登录、注册共用的卡片：品牌、标题、说明，底部放切换链接
defineProps<{ title: string; description?: string }>()
</script>

<template>
  <div class="flex min-h-full items-center justify-center px-4 py-8 font-oi text-sm sm:py-12">
    <div class="w-[min(400px,100%)] rounded-[24px] border border-oi-line bg-[color-mix(in_srgb,var(--oi-raised)_88%,transparent)] px-[30px] pb-[26px] pt-8 shadow-oi-lg backdrop-blur-[18px] max-[420px]:px-5">
      <AppBrand class="pb-[22px]" />
      <h1 class="text-[22px] font-[680] tracking-[-0.02em] text-oi-ink">{{ title }}</h1>
      <p v-if="description" class="mb-[22px] mt-1 text-oi-muted">{{ description }}</p>
      <slot />
      <p v-if="$slots.footer" class="mt-[18px] text-center text-xs text-oi-muted">
        <slot name="footer" />
      </p>
    </div>
  </div>
</template>
