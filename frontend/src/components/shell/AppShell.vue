<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { UiButton, UiSegmented } from '@/components/ui'
import { canAccessRoute, useSession } from '@/composables/useSession'
import { THEME_OPTIONS, useTheme, type ThemePreference } from '@/composables/useTheme'
import { NAV_ITEMS, navItem, type NavItem } from '@/router/nav'
import AppBrand from './AppBrand.vue'
import SiteFooter from './SiteFooter.vue'
import UserMenu from './UserMenu.vue'
import { useLogout } from './useLogout'

// 登录后的应用外壳：桌面端左侧栏 + 内容面板，手机端（900px 以下）改为底部标签栏
const route = useRoute()
const { user, mustChangePassword } = useSession()
const { preference, isDark, setTheme, toggleTheme } = useTheme()
const { logout, pending: loggingOut } = useLogout()

const allowed = (item: NavItem) => canAccessRoute(item.name, item.permissions, user.value)
const workspaceItems = computed(() => NAV_ITEMS.filter(item => item.group === 'workspace' && allowed(item)))
const adminItems = computed(() => NAV_ITEMS.filter(item => item.group === 'admin' && allowed(item)))
const account = navItem('Account')

const isActive = (item: NavItem) => route.path === item.path || route.path.startsWith(`${item.path}/`)
const inAdmin = computed(() => adminItems.value.some(isActive))
const onAccount = computed(() => isActive(account))

// 手机底部标签栏：图库、统计、上传（中间）、管理、我的
const tabs = computed(() => {
  if (mustChangePassword.value) return []
  const byName = (name: string) => workspaceItems.value.find(item => item.name === name)
  return [byName('Gallery'), byName('Stats'), byName('Upload')].filter((item): item is NavItem => !!item)
})

const themeLabel = computed(() => isDark.value ? '切换到浅色' : '切换到深色')
const theme = computed({
  get: () => preference.value,
  set: (value: string) => setTheme(value as ThemePreference),
})
const themeOptions = THEME_OPTIONS.map(option => ({ value: option.value, label: option.label }))

const navItemClass = (item: NavItem) => [
  'flex h-9 items-center gap-2.5 rounded-[9px] px-2.5 font-medium transition-colors duration-150 [&>i]:text-lg [&>i]:transition-colors',
  'focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring',
  isActive(item)
    ? 'bg-oi-sheet text-oi-ink shadow-[var(--oi-shadow-sm),0_0_0_1px_var(--oi-line)] [&>i]:text-oi-accent'
    : 'text-oi-text hover:bg-oi-hover hover:text-oi-ink [&>i]:text-oi-muted',
]
</script>

<template>
  <!-- flow-root 防止内容面板的外边距折叠到外层，露出页面底色 -->
  <div class="flow-root min-h-dvh bg-oi-bg text-oi-text">
    <!-- 桌面端侧栏 -->
    <aside class="fixed inset-y-0 left-0 z-30 hidden w-[236px] flex-col overflow-y-auto px-3 pb-3 pt-3.5 font-oi text-sm max-[1080px]:w-[212px] min-[900px]:flex">
      <AppBrand class="px-2 pb-[18px] pt-1.5" />

      <nav v-if="!mustChangePassword" class="flex flex-col gap-0.5" aria-label="工作区">
        <RouterLink
          v-for="item in workspaceItems"
          :key="item.name"
          :to="item.path"
          :class="navItemClass(item)"
          :aria-current="isActive(item) ? 'page' : undefined"
        >
          <i :class="item.icon" aria-hidden="true" />{{ item.label }}
        </RouterLink>
      </nav>

      <template v-if="adminItems.length">
        <div class="px-2.5 pb-1.5 pt-4 text-[11.5px] tracking-[0.02em] text-oi-faint">管理</div>
        <nav class="flex flex-col gap-0.5" aria-label="管理">
          <RouterLink
            v-for="item in adminItems"
            :key="item.name"
            :to="item.path"
            :class="navItemClass(item)"
            :aria-current="isActive(item) ? 'page' : undefined"
          >
            <i :class="item.icon" aria-hidden="true" />{{ item.label }}
          </RouterLink>
        </nav>
      </template>

      <div v-if="!mustChangePassword" class="px-2.5 pb-1.5 pt-4 text-[11.5px] tracking-[0.02em] text-oi-faint">个人</div>
      <nav class="flex flex-col gap-0.5" aria-label="个人">
        <RouterLink :to="account.path" :class="navItemClass(account)" :aria-current="onAccount ? 'page' : undefined">
          <i :class="account.icon" aria-hidden="true" />{{ account.label }}
        </RouterLink>
      </nav>

      <div class="mt-auto flex flex-col gap-2.5 pt-4">
        <SiteFooter />
        <div class="flex items-center gap-1">
          <div class="min-w-0 flex-1"><UserMenu /></div>
          <UiButton
            variant="ghost"
            icon
            class="!text-[17px] text-oi-muted hover:text-oi-ink"
            :title="themeLabel"
            :aria-label="themeLabel"
            @click="toggleTheme"
          >
            <i :class="isDark ? 'ri-sun-line' : 'ri-moon-line'" aria-hidden="true" />
          </UiButton>
        </div>
      </div>
    </aside>

    <!-- 内容面板；旧页面迁移完成前仍使用页面滚动，不在面板内部滚动 -->
    <div class="min-[900px]:pl-[212px] min-[1081px]:pl-[236px]">
      <main
        class="flex min-h-dvh min-w-0 flex-col bg-oi-sheet pb-[calc(64px+env(safe-area-inset-bottom))] min-[900px]:m-2 min-[900px]:ml-0 min-[900px]:min-h-[calc(100dvh-16px)] min-[900px]:rounded-oi-lg min-[900px]:border min-[900px]:border-oi-line min-[900px]:pb-0 min-[900px]:shadow-oi"
      >
        <!-- 手机端：管理页面之间的切换 -->
        <nav
          v-if="inAdmin"
          class="flex flex-none gap-1.5 overflow-x-auto px-4 pt-3 font-oi [scrollbar-width:none] min-[900px]:hidden"
          aria-label="管理"
        >
          <RouterLink
            v-for="item in adminItems"
            :key="item.name"
            :to="item.path"
            :aria-current="isActive(item) ? 'page' : undefined"
            :class="[
              'inline-flex h-[34px] flex-none items-center gap-1.5 rounded-full border px-3 text-[13px] font-[550] [&>i]:text-[15px]',
              isActive(item) ? 'border-transparent bg-oi-btn text-oi-btn-ink' : 'border-oi-line bg-oi-raised text-oi-text',
            ]"
          >
            <i :class="item.icon" aria-hidden="true" />{{ item.label }}
          </RouterLink>
        </nav>

        <!-- 手机端「我的」：主题、退出登录和页脚（桌面端在侧栏底部） -->
        <section v-if="onAccount" class="px-4 pt-4 font-oi min-[900px]:hidden" aria-label="外观与登录">
          <div class="divide-y divide-oi-line rounded-oi border border-oi-line bg-oi-raised">
            <div class="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
              <span id="mobile-theme-label" class="text-sm font-semibold text-oi-ink">外观</span>
              <UiSegmented v-model="theme" :options="themeOptions" aria-labelledby="mobile-theme-label" />
            </div>
            <div class="px-4 py-3">
              <UiButton variant="danger" :disabled="loggingOut" @click="logout">
                <i class="ri-logout-box-r-line" aria-hidden="true" />退出登录
              </UiButton>
            </div>
          </div>
          <SiteFooter class="mt-3.5" />
        </section>

        <div class="min-w-0 flex-1 px-4 pb-8 pt-4 min-[900px]:px-6 min-[900px]:pb-10 min-[900px]:pt-6 xl:px-8">
          <slot />
        </div>
      </main>
    </div>

    <!-- 手机端底部标签栏 -->
    <nav
      class="fixed inset-x-0 bottom-0 z-[80] flex h-[calc(62px+env(safe-area-inset-bottom))] border-t border-oi-line bg-[color-mix(in_srgb,var(--oi-sheet)_86%,transparent)] px-2 pb-[env(safe-area-inset-bottom)] pt-1.5 font-oi backdrop-blur-[16px] backdrop-saturate-[1.4] min-[900px]:hidden"
      aria-label="主导航"
    >
      <RouterLink
        v-for="item in tabs"
        :key="item.name"
        :to="item.path"
        :aria-current="isActive(item) ? 'page' : undefined"
        :class="[
          'flex flex-1 flex-col items-center justify-center gap-0.5 rounded-xl text-[11px] max-[380px]:text-[10.5px]',
          isActive(item) ? 'text-oi-ink' : 'text-oi-muted',
          item.name === 'Upload'
            ? (isActive(item) ? '[&>i]:bg-oi-accent' : '[&>i]:bg-oi-btn')
            : (isActive(item) ? '[&>i]:text-oi-accent' : ''),
          item.name === 'Upload' ? '[&>i]:grid [&>i]:h-8 [&>i]:w-11 [&>i]:place-items-center [&>i]:rounded-[11px] [&>i]:text-xl [&>i]:text-oi-btn-ink' : '[&>i]:text-[22px]',
        ]"
      >
        <i :class="item.name === 'Upload' ? 'ri-add-line' : item.icon" aria-hidden="true" />{{ item.label }}
      </RouterLink>
      <RouterLink
        v-if="adminItems.length && !mustChangePassword"
        :to="adminItems[0].path"
        :aria-current="inAdmin ? 'page' : undefined"
        :class="['flex flex-1 flex-col items-center justify-center gap-0.5 rounded-xl text-[11px] max-[380px]:text-[10.5px] [&>i]:text-[22px]', inAdmin ? 'text-oi-ink [&>i]:text-oi-accent' : 'text-oi-muted']"
      >
        <i class="ri-dashboard-line" aria-hidden="true" />管理
      </RouterLink>
      <RouterLink
        :to="account.path"
        :aria-current="onAccount ? 'page' : undefined"
        :class="['flex flex-1 flex-col items-center justify-center gap-0.5 rounded-xl text-[11px] max-[380px]:text-[10.5px] [&>i]:text-[22px]', onAccount ? 'text-oi-ink [&>i]:text-oi-accent' : 'text-oi-muted']"
      >
        <i class="ri-user-3-line" aria-hidden="true" />我的
      </RouterLink>
    </nav>
  </div>
</template>
