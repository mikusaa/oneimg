<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import AppBrand from './AppBrand.vue'
import SiteFooter from './SiteFooter.vue'

// 登录、注册等公开页面：日夜切换和页脚。登录、注册（meta.auth）的品牌放在卡片里，背景加柔和的光晕
const route = useRoute()
const { isDark, toggleTheme } = useTheme()
const themeLabel = computed(() => isDark.value ? '切换到浅色' : '切换到深色')
const auth = computed(() => !!route.meta.auth)
</script>

<template>
  <div
    :class="[
      'flex min-h-dvh flex-col bg-oi-bg text-oi-text',
      auth && 'bg-[radial-gradient(60%_60%_at_15%_20%,color-mix(in_srgb,var(--oi-accent)_16%,transparent),transparent_70%),radial-gradient(60%_60%_at_85%_80%,color-mix(in_srgb,var(--oi-sakura)_18%,transparent),transparent_70%)]',
    ]"
  >
    <header :class="['flex items-center gap-3 px-4 pt-4 font-oi sm:px-6', auth ? 'justify-end' : 'justify-between']">
      <AppBrand v-if="!auth" />
      <button
        type="button"
        class="grid h-9 w-9 place-items-center rounded-[10px] text-[17px] text-oi-muted transition-colors hover:bg-oi-hover hover:text-oi-ink focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
        :title="themeLabel"
        :aria-label="themeLabel"
        @click="toggleTheme"
      >
        <i :class="isDark ? 'ri-sun-line' : 'ri-moon-line'" aria-hidden="true" />
      </button>
    </header>
    <main class="flex-1">
      <slot />
    </main>
    <footer class="pb-4 pt-2 text-center font-oi [&>div>div]:justify-center">
      <SiteFooter />
    </footer>
  </div>
</template>
