import { computed, readonly, ref, type DeepReadonly } from 'vue'
import { getStoredUser, hasAnyPermission, isSuperAdmin, ROLE_ADMIN } from '@/utils/permissions.ts'

export interface SessionUser {
  id?: number
  username?: string
  role?: number
  password_change_required?: boolean
  bootstrap_verification_required?: boolean
  permission?: { codes?: string[]; bucket_ids?: number[] }
}

const STORAGE_KEY = 'userInfo'

// 登录信息仍存在 localStorage 的 userInfo 里（旧页面直接读取它），这里额外保留一份响应式副本给外壳使用
const user = ref<SessionUser>(getStoredUser())

/** 由路由守卫在每次校验 /me 之后调用 */
export const setSessionUser = (value: SessionUser) => {
  user.value = value
  localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
}

export const clearSession = () => {
  user.value = {}
  localStorage.removeItem(STORAGE_KEY)
}

/**
 * 页面权限：带 permissions 的页面只对拥有其中任一权限码的管理员开放；
 * 需要改密的账户只能进入账户设置。
 */
export const canAccessRoute = (routeName: string | undefined, permissions: readonly string[] | undefined, value: DeepReadonly<SessionUser>) => {
  if (value.password_change_required) return routeName === 'Account'
  if (!permissions?.length) return true
  return Number(value.role) === ROLE_ADMIN && hasAnyPermission(permissions, value)
}

export const useSession = () => {
  const roleLabel = computed(() => {
    if (isSuperAdmin(user.value)) return '超级管理员'
    return Number(user.value.role) === ROLE_ADMIN ? '管理员' : '普通用户'
  })
  const mustChangePassword = computed(() => !!user.value.password_change_required)
  return { user: readonly(user), roleLabel, mustChangePassword }
}
