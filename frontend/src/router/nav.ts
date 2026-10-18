// 侧栏、手机底部标签栏和路由共用的页面清单：导航名称统一两个字，页面标题可以更完整
export type NavGroup = 'workspace' | 'admin' | 'personal'

export interface NavItem {
  name: string
  path: string
  /** 导航里显示的名称 */
  label: string
  /** 页面标题，用于浏览器标签 */
  title: string
  icon: string
  group: NavGroup
  /** 拥有其中任一权限码的管理员可以访问 */
  permissions?: string[]
  /** 单键快捷键，由外壳统一处理 */
  shortcut?: string
}

export const NAV_ITEMS: NavItem[] = [
  { name: 'Upload', path: '/upload', label: '上传', title: '上传', icon: 'ri-upload-cloud-2-line', group: 'workspace', shortcut: 'U' },
  { name: 'Gallery', path: '/gallery', label: '图库', title: '图库', icon: 'ri-image-2-line', group: 'workspace', shortcut: 'G' },
  { name: 'Stats', path: '/stats', label: '统计', title: '统计', icon: 'ri-bar-chart-box-line', group: 'workspace' },
  {
    name: 'Storage', path: '/storage', label: '存储', title: '存储', icon: 'ri-database-2-line', group: 'admin',
    permissions: ['storage:create', 'storage:update', 'storage:delete'],
  },
  { name: 'Users', path: '/users', label: '用户', title: '用户', icon: 'ri-group-line', group: 'admin', permissions: ['user:list'] },
  {
    name: 'Settings', path: '/settings', label: '设置', title: '系统设置', icon: 'ri-settings-3-line', group: 'admin',
    permissions: ['setting:upload', 'setting:image', 'setting:security', 'setting:seo'],
  },
  { name: 'Account', path: '/account', label: '账户', title: '账户设置', icon: 'ri-user-settings-line', group: 'personal' },
]

export const navItem = (name: string) => {
  const item = NAV_ITEMS.find(entry => entry.name === name)
  if (!item) throw new Error(`未定义的导航项：${name}`)
  return item
}
