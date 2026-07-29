<template>
  <header
    class="app-material fixed inset-x-0 top-0 z-40 border-b border-slate-200/70 bg-white/80 backdrop-blur-xl dark:border-white/10 dark:bg-slate-950/80"
    :class="{ 'lg:left-[var(--app-sidebar-width)]': showSidebar }"
  >
    <div class="mx-auto flex h-[var(--app-header-height-mobile)] items-center justify-between gap-2 px-2.5 sm:px-4 md:h-[var(--app-header-height)] md:px-5 xl:px-6 2xl:px-8">
      <div class="flex min-w-0 flex-1 items-center gap-2 sm:gap-2.5">
        <button
          v-if="showSidebar"
          ref="sidebarToggleRef"
          type="button"
          class="pressable inline-flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-700 hover:border-slate-300 hover:text-slate-900 dark:border-white/10 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-white/20 dark:hover:text-white lg:hidden"
          aria-label="打开导航"
          @click="toggleSidebar"
        >
          <i class="ri-menu-3-line text-lg"></i>
        </button>
        <div
          class="flex min-w-0 items-center gap-2 sm:gap-2.5"
          :class="{ 'lg:hidden': showSidebar }"
        >
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-slate-900 text-sm font-bold text-white shadow-sm dark:bg-white dark:text-slate-900 sm:h-9 sm:w-9 sm:text-base">
              {{ getFirstWord(seoTitle) }}
          </div>
          <div class="min-w-0">
            <h1 class="truncate text-[13px] font-semibold text-slate-900 dark:text-slate-100 sm:text-[15px] md:text-base xl:text-lg">{{ seoTitle }}</h1>
          </div>
        </div>
      </div>

      <div class="flex shrink-0 items-center gap-1.5 md:gap-2">
        <div ref="themeMenuRef" class="relative">
          <button
            ref="themeMenuButtonRef"
            type="button"
            class="pressable inline-flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-600 hover:border-slate-300 hover:text-slate-900 dark:border-white/10 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-white/20 dark:hover:text-white md:w-auto md:gap-1.5"
            :aria-label="`主题：${activeThemeOption.label}`"
            :aria-expanded="themeMenuOpen"
            :title="`主题：${activeThemeOption.label}`"
            aria-haspopup="menu"
            @click="toggleThemeMenu"
          >
            <i :class="activeThemeOption.icon"></i>
            <span class="hidden md:inline">{{ activeThemeOption.label }}</span>
            <i class="ri-arrow-down-s-line hidden text-base opacity-60 md:inline" aria-hidden="true"></i>
          </button>

          <transition name="menu-pop">
            <div
              v-if="themeMenuOpen"
              class="app-material absolute right-0 top-[calc(100%+8px)] z-50 w-40 origin-top-right overflow-hidden rounded-lg border border-slate-200 bg-white/95 p-1.5 shadow-xl backdrop-blur-xl dark:border-white/10 dark:bg-slate-900/95"
              role="menu"
              aria-label="主题设置"
            >
              <button
                v-for="option in themeOptions"
                :key="option.value"
                type="button"
                class="pressable flex min-h-10 w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm"
                :class="themePreference === option.value
                  ? 'bg-slate-100 font-medium text-slate-900 dark:bg-white/10 dark:text-white'
                  : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-white/5 dark:hover:text-white'"
                role="menuitemradio"
                :aria-checked="themePreference === option.value"
                @click="selectTheme(option.value)"
              >
                <i :class="option.icon" class="text-base"></i>
                <span class="flex-1">{{ option.label }}</span>
                <i v-if="themePreference === option.value" class="ri-check-line text-base" aria-hidden="true"></i>
              </button>
            </div>
          </transition>
        </div>

        <button
          v-if="isLogin && showSidebar"
          type="button"
          class="pressable inline-flex h-10 w-10 items-center justify-center rounded-lg border border-red-200 bg-white px-3 py-1.5 text-sm font-medium text-red-600 hover:border-red-300 hover:bg-red-50 hover:text-red-800 dark:border-red-900/60 dark:bg-red-950/40 dark:text-red-300 dark:hover:border-red-800 dark:hover:bg-red-950/70 md:w-auto md:gap-1.5"
          aria-label="退出登录"
          title="退出登录"
          @click="handleLogout"
        >
          <i class="ri-logout-circle-r-line"></i>
        </button>
      </div>
    </div>
  </header>

  <aside
    v-if="showSidebar"
    class="sidebar-panel app-material fixed inset-y-0 left-0 z-50 w-[min(88vw,var(--app-sidebar-width))] border-r border-slate-200/80 bg-white/95 px-4 py-4 shadow-xl backdrop-blur-xl dark:border-white/10 dark:bg-slate-950/95 lg:w-[var(--app-sidebar-width)] lg:shadow-none"
    :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'"
  >
    <div class="flex h-full flex-col overflow-hidden">
      <div class="flex min-h-9 items-center justify-between border-b border-slate-200/70 pb-3 dark:border-white/10 sm:pb-3.5">
        <router-link
          to="/"
          class="flex min-w-0 items-center gap-2.5"
          title="返回控制中心"
          @click="handleNavClick"
        >
          <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-slate-900 text-base font-bold text-white shadow-sm dark:bg-white dark:text-slate-900">
            {{ getFirstWord(seoTitle) }}
          </span>
          <span class="truncate text-base font-semibold text-slate-900 dark:text-slate-100">{{ seoTitle }}</span>
        </router-link>
        <button
          ref="sidebarCloseRef"
          type="button"
          class="pressable inline-flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 hover:text-slate-900 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800 dark:hover:text-white lg:hidden"
          aria-label="关闭导航"
          @click="closeSidebar"
        >
          <i class="ri-close-line"></i>
        </button>
      </div>

      <nav class="mt-3 flex-1 overflow-y-auto pr-1 sm:mt-4">
        <ul class="space-y-1.5">
          <li v-for="item in navItems" :key="item.path">
            <router-link
              :to="item.path"
              class="pressable group flex min-h-11 items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium sm:px-3.5 sm:py-2.5"
              :class="isRouteActive(item.path) ? 'bg-slate-900 text-white dark:bg-slate-800 dark:text-white dark:ring-1 dark:ring-inset dark:ring-white/10' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-white/5 dark:hover:text-white'"
              @click="handleNavClick"
            >
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-lg text-base sm:h-9 sm:w-9"
                :class="isRouteActive(item.path) ? 'bg-white/15 text-white dark:bg-primary/20 dark:text-blue-300' : 'bg-slate-100 text-slate-500 group-hover:bg-white group-hover:text-slate-900 dark:bg-white/5 dark:text-slate-400 dark:group-hover:bg-slate-800 dark:group-hover:text-white'">
                <i :class="`ri-${item.icon}`"></i>
              </span>
              <span class="flex-1">{{ item.name }}</span>
              <i class="ri-arrow-right-s-line text-base opacity-40"></i>
            </router-link>
          </li>
        </ul>
      </nav>
    </div>
  </aside>

  <transition name="fade">
    <div v-if="showSidebar && sidebarOpen" class="app-scrim fixed inset-0 z-40 bg-slate-950/45 backdrop-blur-[2px] lg:hidden" @click="closeSidebar"></div>
  </transition>
</template>

<script setup>
import { computed, nextTick, ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import Message from '@/utils/message.js'
import { getStoredUser, hasAnyPermission, hasPermission, ROLE_ADMIN } from '@/utils/permissions.js'
import { lockBodyScroll, unlockBodyScroll } from '@/utils/scrollLock.js'

const router = useRouter()
const route = useRoute()

const seoTitle = ref('初春图床')
const isLogin = ref(false)
const isDark = ref(false)
const themePreference = ref('system')
const themeMenuOpen = ref(false)
const themeMenuRef = ref(null)
const themeMenuButtonRef = ref(null)
const sidebarOpen = ref(false)
const sidebarToggleRef = ref(null)
const sidebarCloseRef = ref(null)
const navItems = ref([])
const storageKey = 'theme-preference'
const themeOptions = [
  { value: 'system', label: '跟随设备', icon: 'ri-computer-line' },
  { value: 'light', label: '浅色', icon: 'ri-sun-line' },
  { value: 'dark', label: '深色', icon: 'ri-moon-clear-line' },
]
const validThemes = new Set(themeOptions.map(option => option.value))
let colorSchemeMediaQuery = null
const activeRoutePath = computed(() => route.matched.length > 0 ? route.path : window.location.pathname)
const showSidebar = computed(() => isLogin.value && !route.meta.public)
const activeThemeOption = computed(() => {
  return themeOptions.find(option => option.value === themePreference.value) || themeOptions[0]
})

const refreshNavItems = () => {
  const userInfo = getStoredUser()
  navItems.value = []
  isLogin.value = !!userInfo.username

  if (!isLogin.value) {
    navItems.value.push({ path: '/login', icon: 'login-circle-line', name: '登录' })
    return
  }

  navItems.value.push(
    { path: '/', icon: 'home-5-line', name: '控制中心' },
    { path: '/gallery', icon: 'gallery-view-2', name: '图库管理' },
    { path: '/stats', icon: 'bar-chart-grouped-line', name: '数据统计' }
  )

  if (Number(userInfo?.role) === ROLE_ADMIN) {
    if (hasAnyPermission(['tag:create', 'tag:update', 'tag:delete'], userInfo)) {
      navItems.value.push({ path: '/tags', icon: 'price-tag-3-line', name: '标签管理' })
    }
    if (hasAnyPermission(['storage:create', 'storage:update', 'storage:delete'], userInfo)) {
      navItems.value.push({ path: '/buckets', icon: 'database-2-line', name: '存储管理' })
    }
    if (hasPermission('user:list', userInfo)) {
      navItems.value.push({ path: '/users', icon: 'user-line', name: '用户管理' })
    }
    if (hasAnyPermission(['setting:upload', 'setting:image', 'setting:security', 'setting:api', 'setting:seo'], userInfo)) {
      navItems.value.push({ path: '/settings', icon: 'settings-4-line', name: '系统设置' })
    }
  }

  navItems.value.push({ path: '/account', icon: 'shield-user-line', name: '账户设置' })
}

const isRouteActive = (targetPath) => {
  const exactMatchPaths = ['/', '/login']
  if (exactMatchPaths.includes(targetPath)) {
    return activeRoutePath.value === targetPath
  }
  return activeRoutePath.value.startsWith(targetPath)
}

const getFirstWord = (title) => {
  if (!title) return '图'
  return title.trim().slice(0, 1)
}

const detectUserThemePreference = () => {
  const savedTheme = typeof localStorage !== 'undefined' ? localStorage.getItem(storageKey) : ''
  return validThemes.has(savedTheme) ? savedTheme : 'system'
}

const applyResolvedTheme = () => {
  const htmlElement = document.documentElement
  isDark.value = themePreference.value === 'dark'
    || (themePreference.value === 'system' && colorSchemeMediaQuery?.matches)

  if (isDark.value) {
    htmlElement.classList.add('dark')
  } else {
    htmlElement.classList.remove('dark')
  }
  htmlElement.style.colorScheme = isDark.value ? 'dark' : 'light'
}

const applyTheme = (theme) => {
  themePreference.value = validThemes.has(theme) ? theme : 'system'
  localStorage.setItem(storageKey, themePreference.value)
  applyResolvedTheme()
}

const toggleThemeMenu = async () => {
  themeMenuOpen.value = !themeMenuOpen.value
  if (themeMenuOpen.value) {
    await nextTick()
    themeMenuRef.value?.querySelector('[role="menuitemradio"]')?.focus()
  }
}

const selectTheme = (theme) => {
  applyTheme(theme)
  themeMenuOpen.value = false
  themeMenuButtonRef.value?.focus()
}

const handleSystemThemeChange = () => {
  if (themePreference.value === 'system') {
    applyResolvedTheme()
  }
}

const handleDocumentClick = (event) => {
  if (themeMenuOpen.value && !themeMenuRef.value?.contains(event.target)) {
    themeMenuOpen.value = false
  }
}

const handleDocumentKeydown = (event) => {
  if (event.key === 'Escape') {
    if (themeMenuOpen.value) {
      themeMenuOpen.value = false
      themeMenuButtonRef.value?.focus()
      return
    }
    if (sidebarOpen.value) closeSidebar(true)
  }
}

const openSidebar = async () => {
  if (sidebarOpen.value) return
  sidebarOpen.value = true
  lockBodyScroll()
  await nextTick()
  sidebarCloseRef.value?.focus()
}

const closeSidebar = (restoreFocus = false) => {
  if (!sidebarOpen.value) return
  sidebarOpen.value = false
  unlockBodyScroll()
  if (restoreFocus) nextTick(() => sidebarToggleRef.value?.focus())
}

const toggleSidebar = () => {
  if (sidebarOpen.value) {
    closeSidebar()
  } else {
    openSidebar()
  }
}

const handleNavClick = () => {
  if (window.innerWidth < 1024) {
    closeSidebar()
  }
}

const handleLogout = async () => {
  localStorage.removeItem('token')
  localStorage.removeItem('userInfo')
  try {
    await fetch('/api/logout', { method: 'POST' })
    Message.success('登出成功')
    refreshNavItems()
    router.push('/login').catch(() => {})
  } catch (error) {
    Message.error('登出失败')
  }
}

const handleSeoUpdate = (data) => {
  if (data?.seo_title) {
    seoTitle.value = data.seo_title
  }
}

const handleResize = () => {
  if (window.innerWidth >= 1024) {
    closeSidebar()
  }
}

onMounted(() => {
  colorSchemeMediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  applyTheme(detectUserThemePreference())
  if (typeof colorSchemeMediaQuery.addEventListener === 'function') {
    colorSchemeMediaQuery.addEventListener('change', handleSystemThemeChange)
  } else {
    colorSchemeMediaQuery.addListener(handleSystemThemeChange)
  }
  refreshNavItems()
  window.refreshNavItems = refreshNavItems
  window.seoBus?.onUpdate(handleSeoUpdate)
  if (window.seoStting?.seo_title) {
    seoTitle.value = window.seoStting.seo_title
  }
  window.addEventListener('resize', handleResize)
  document.addEventListener('click', handleDocumentClick)
  document.addEventListener('keydown', handleDocumentKeydown)
})

onUnmounted(() => {
  if (window.seoBus?.callbacks) {
    window.seoBus.callbacks = window.seoBus.callbacks.filter((cb) => cb !== handleSeoUpdate)
  }
  if (typeof colorSchemeMediaQuery?.removeEventListener === 'function') {
    colorSchemeMediaQuery.removeEventListener('change', handleSystemThemeChange)
  } else {
    colorSchemeMediaQuery?.removeListener(handleSystemThemeChange)
  }
  window.removeEventListener('resize', handleResize)
  document.removeEventListener('click', handleDocumentClick)
  document.removeEventListener('keydown', handleDocumentKeydown)
  if (sidebarOpen.value) unlockBodyScroll()
  delete window.refreshNavItems
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.sidebar-panel {
  transition: transform 260ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.menu-pop-enter-active,
.menu-pop-leave-active {
  transition: opacity 160ms cubic-bezier(0.2, 0.8, 0.2, 1), transform 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.menu-pop-enter-from,
.menu-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

@media (prefers-reduced-motion: reduce) {
  .menu-pop-enter-from,
  .menu-pop-leave-to {
    transform: none;
  }
}
</style>
