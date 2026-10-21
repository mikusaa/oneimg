<script setup lang="ts">
import { computed, ref } from 'vue'
import { UiAvatar, UiMenu, UiMenuItem, UiMenuLabel, UiMenuSegmented, UiMenuSeparator } from '@/components/ui'
import { useSession } from '@/composables/useSession'
import { THEME_OPTIONS, useTheme, type ThemePreference } from '@/composables/useTheme'
import { useLogout } from './useLogout'

const { user, roleLabel, mustChangePassword } = useSession()
const { preference, setTheme } = useTheme()
const { logout } = useLogout()

const open = ref(false)
const themeOptions = THEME_OPTIONS.map(option => ({ value: option.value, icon: option.icon, title: option.label }))
const theme = computed({
  get: () => preference.value,
  set: (value: string) => setTheme(value as ThemePreference),
})
</script>

<template>
  <UiMenu v-model:open="open" side="top" :side-offset="8" match-trigger-width>
    <template #trigger>
      <button
        type="button"
        class="flex w-full min-w-0 items-center gap-2.5 rounded-oi p-2 text-left transition-colors duration-150 hover:bg-oi-hover focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring aria-expanded:bg-oi-hover"
      >
        <UiAvatar :name="user.username" :src="user.avatar_url" />
        <span class="min-w-0 flex-1">
          <span class="block truncate text-[13.5px] font-semibold leading-tight text-oi-ink">{{ user.username }}</span>
          <span class="block text-xs text-oi-muted">{{ roleLabel }}</span>
        </span>
        <i class="ri-expand-up-down-line text-oi-muted" aria-hidden="true" />
      </button>
    </template>

    <UiMenuItem to="/account" icon="ri-user-settings-line">{{ mustChangePassword ? '修改密码' : '账户设置' }}</UiMenuItem>
    <UiMenuSeparator />
    <UiMenuLabel>外观</UiMenuLabel>
    <UiMenuSegmented v-model="theme" :options="themeOptions" aria-label="主题" />
    <UiMenuSeparator />
    <UiMenuItem icon="ri-logout-box-r-line" danger @select="logout">退出登录</UiMenuItem>
  </UiMenu>
</template>
