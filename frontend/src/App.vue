<template>
  <component :is="layout" v-if="layout">
    <router-view v-slot="{ Component, route: currentRoute }">
      <transition name="route-view" mode="out-in">
        <component :is="Component" :key="currentRoute.name" class="route-view" />
      </transition>
    </router-view>
  </component>

  <!-- 新界面的提示条和确认弹窗，页面里调用 toast() / confirm() -->
  <UiToaster />
  <UiConfirmHost />
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'
import PublicLayout from '@/components/shell/PublicLayout.vue'
import '@/composables/useTheme'
// 异步加载，避免 Reka UI 进入首屏主包；登录页不加载应用外壳。加载前调用的 toast() / confirm() 会在加载后显示
const AppShell = defineAsyncComponent(() => import('@/components/shell/AppShell.vue'))
const UiToaster = defineAsyncComponent(() => import('@/components/ui/UiToaster.vue'))
const UiConfirmHost = defineAsyncComponent(() => import('@/components/ui/UiConfirmHost.vue'))

const route = useRoute()
// 首次导航完成前不渲染，避免登录页闪出应用外壳
const layout = computed(() => {
  if (!route.matched.length) return null
  return route.meta.public ? PublicLayout : AppShell
})
</script>
