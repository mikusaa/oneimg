/// <reference types="vite/client" />

declare module '*.vue' {
  const component: any
  export default component
}

declare module '*.js'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
    /** 不需要登录，也不显示应用外壳 */
    public?: boolean
    /** 拥有其中任一权限码的管理员可以访问 */
    permissions?: string[]
  }
}

declare global {
  interface Window {
    Spotlight?: { show: (items: Array<{ src: string }>) => void }
  }
}

export {}
