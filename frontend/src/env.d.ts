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
    /** 页面自己控制内边距，内容铺满内容面板（如上传页的右侧栏） */
    bleed?: boolean
    /** 登录、注册：公开页面里品牌放在卡片中，背景加光晕 */
    auth?: boolean
  }
}

export {}
