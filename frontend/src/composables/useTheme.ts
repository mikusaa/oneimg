import { computed, ref, watchEffect } from 'vue'

export type ThemePreference = 'system' | 'light' | 'dark'

// 与 index.html 的首屏脚本使用同一个键和取值，升级后用户原来的主题设置不会丢失
const STORAGE_KEY = 'theme-preference'
const PREFERENCES: ThemePreference[] = ['system', 'light', 'dark']

export const THEME_OPTIONS: Array<{ value: ThemePreference; label: string; icon: string }> = [
  { value: 'system', label: '跟随系统', icon: 'ri-computer-line' },
  { value: 'light', label: '浅色', icon: 'ri-sun-line' },
  { value: 'dark', label: '深色', icon: 'ri-moon-line' },
]

const isPreference = (value: unknown): value is ThemePreference => PREFERENCES.includes(value as ThemePreference)

const readPreference = (): ThemePreference => {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    return isPreference(saved) ? saved : 'system'
  } catch {
    return 'system'
  }
}

const preference = ref<ThemePreference>(readPreference())
const media = window.matchMedia('(prefers-color-scheme: dark)')
const systemDark = ref(media.matches)
media.addEventListener('change', event => { systemDark.value = event.matches })

const isDark = computed(() => preference.value === 'dark' || (preference.value === 'system' && systemDark.value))

// 全局只有一份主题状态，通过 html.dark 生效
watchEffect(() => {
  const root = document.documentElement
  root.classList.toggle('dark', isDark.value)
  root.style.colorScheme = isDark.value ? 'dark' : 'light'
})

const setTheme = (value: ThemePreference) => {
  preference.value = isPreference(value) ? value : 'system'
  try {
    localStorage.setItem(STORAGE_KEY, preference.value)
  } catch {
    // 无法写入时只在本次访问内生效
  }
}

/** 侧栏的日夜切换：按当前实际显示的主题在浅色和深色之间切换 */
const toggleTheme = () => setTheme(isDark.value ? 'light' : 'dark')

export const useTheme = () => ({ preference, isDark, setTheme, toggleTheme })
