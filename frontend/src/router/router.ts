import { apiFetch } from "@/api/client.ts"
import { createRouter, createWebHistory, type RouteComponent, type RouteLocationNormalized, type RouteMeta, type RouteRecordRaw } from 'vue-router'
import { canAccessRoute, setSessionUser, clearSession, type SessionUser } from '@/composables/useSession'
import { loadSiteConfig, useSite } from '@/composables/useSite'
import { navItem } from './nav'

const site = useSite()

const navRoute = (name: string, component: () => Promise<RouteComponent>, meta: RouteMeta = {}): RouteRecordRaw => {
  const item = navItem(name)
  return { path: item.path, name, component, meta: { title: item.title, permissions: item.permissions, ...meta } }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { title: '登录', public: true, auth: true }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register.vue'),
    meta: { title: '注册', public: true, auth: true }
  },
  navRoute('Upload', () => import('@/views/Upload.vue'), { bleed: true }),
  navRoute('Gallery', () => import('@/views/Gallery.vue'), { bleed: true }),
  navRoute('Stats', () => import('@/views/Stats.vue')),
  navRoute('Storage', () => import('@/views/Storage.vue')),
  navRoute('Users', () => import('@/views/Users.vue')),
  navRoute('Settings', () => import('@/views/Settings.vue')),
  navRoute('Account', () => import('@/views/Account.vue')),
  // 旧地址继续可用，避免书签失效
  { path: '/', redirect: '/upload' },
  { path: '/buckets', redirect: to => ({ path: '/storage', query: to.query, hash: to.hash }) },
  { path: '/tags', redirect: to => ({ path: '/gallery', query: { ...to.query, view: 'tags' } }) },
  { path: '/:pathMatch(.*)*', redirect: '/upload' },
]

// 开发环境专用：新界面基础组件预览，生产构建不包含这个页面
if (import.meta.env.DEV) {
  routes.push({
    path: '/_ui',
    name: 'UiShowcase',
    component: () => import('@/views/dev/UiShowcase.vue'),
    meta: { title: '组件预览', public: true }
  })
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes
})

const getPageTitle = (to: RouteLocationNormalized) => to.meta.title ? `${to.meta.title} - ${site.title}` : site.title

router.beforeEach(async (to) => {
  try {
    await loadSiteConfig()
    document.title = getPageTitle(to)

    if (to.meta.public) return true

    // 验证本地登录信息
    let userInfo: SessionUser = {}
    try {
      userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}')
    } catch {
      userInfo = {}
    }
    // 未登录时记下原地址，登录后回到这里
    if (!userInfo.username) {
      clearSession()
      return to.fullPath === '/' ? '/login' : { path: '/login', query: { redirect: to.fullPath } }
    }

    // 向服务器确认登录状态，并刷新角色、权限和改密要求
    const response = await apiFetch('/api/v1/me')
    if (!response.ok) {
      clearSession()
      throw new Error(`登录状态验证失败：${response.status}`)
    }
    const result = await response.json()
    if (!result.data?.username || userInfo.username !== result.data.username) {
      clearSession()
      return '/login'
    }

    const current: SessionUser = {
      ...userInfo,
      id: result.data.id ?? userInfo.id,
      role: result.data.role,
      permission: result.data.permission || { codes: [], bucket_ids: [] },
      password_change_required: result.data.password_change_required,
      bootstrap_verification_required: result.data.bootstrap_verification_required,
    }
    setSessionUser(current)

    // 需要改密时只允许进入账户设置；无权访问的页面回到上传页
    if (!canAccessRoute(String(to.name), to.meta.permissions, current)) {
      return current.password_change_required ? '/account' : '/upload'
    }
    return true
  } catch {
    // 会话失效：回到登录页，登录后返回原地址
    if (to.path === '/login') return true
    return to.fullPath === '/' ? '/login' : { path: '/login', query: { redirect: to.fullPath } }
  }
})

// 被取消的导航（如设置页「留在本页」）也会调用 afterEach，这时不改标题
router.afterEach((to, _from, failure) => {
  if (!failure) document.title = getPageTitle(to)
})

export default router
