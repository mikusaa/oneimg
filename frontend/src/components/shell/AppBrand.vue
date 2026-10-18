<script setup lang="ts">
import { computed } from 'vue'
import { PRODUCT_NAME, useSite } from '@/composables/useSite'
import AppLogo from './AppLogo.vue'

// 品牌区：站点名称来自系统设置（默认 OneImg）；改过名称时下方注明 OneImg；设置了网站图标时用它代替 Logo
const site = useSite()
const renamed = computed(() => site.title !== PRODUCT_NAME)
</script>

<template>
  <div class="flex min-w-0 items-center gap-2.5">
    <span
      v-if="site.icon"
      class="grid h-8 w-8 flex-none place-items-center rounded-[9px] border border-oi-line bg-oi-panel"
    >
      <img :src="site.icon" alt="" class="h-5 w-5 object-contain" />
    </span>
    <span v-else class="flex-none rounded-[9px] shadow-[0_4px_12px_-5px_rgba(16,160,100,0.55)]">
      <AppLogo />
    </span>
    <div class="min-w-0">
      <div class="truncate text-[15px] font-[650] tracking-[-0.01em] text-oi-ink">{{ site.title }}</div>
      <div v-if="renamed" class="text-[11.5px] text-oi-muted">{{ PRODUCT_NAME }}</div>
    </div>
  </div>
</template>
