import { reactive, readonly } from 'vue'
import { apiFetch } from '@/api/client.ts'
import { toast } from '@/components/ui/toast'

/** 默认站点名称；管理员在系统设置里改过名称时，品牌区下方仍注明 OneImg */
export const PRODUCT_NAME = 'OneImg'

export interface SiteConfig {
  title: string
  description: string
  keywords: string
  icp: string
  publicSecurity: string
  icon: string
}

const site = reactive<SiteConfig>({
  title: PRODUCT_NAME,
  description: '',
  keywords: '',
  icp: '',
  publicSecurity: '',
  icon: '',
})

const setMetaTag = (name: string, content: string) => {
  let tag = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  if (!tag) {
    tag = document.createElement('meta')
    tag.name = name
    document.head.appendChild(tag)
  }
  if (content.trim()) tag.content = content.trim()
  else tag.removeAttribute('content')
}

// 设置了网站图标时替换默认的 OneImg Logo，清空后换回来
const defaultFavicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]')?.getAttribute('href') ?? ''
const applyFavicon = (value: string) => {
  const href = value || defaultFavicon
  if (!href) return
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.removeAttribute('type')
  link.href = href
}

let loading: Promise<Readonly<SiteConfig>> | null = null

/** 读取公开的站点信息，整个页面生命周期只请求一次；失败时沿用默认值 */
export const loadSiteConfig = () => {
  loading ??= (async () => {
    try {
      const response = await apiFetch('/api/v1/public/config')
      if (!response.ok) throw new Error(`请求失败：${response.status} ${response.statusText}`)
      const result = await response.json()
      const data = result?.data?.site
      if (!data) throw new Error(result?.detail || '获取站点信息失败：无数据')
      Object.assign(site, {
        title: data.title || site.title,
        description: data.description || '',
        keywords: data.keywords || '',
        icp: data.icp || '',
        publicSecurity: data.public_security || '',
        icon: data.icon || '',
      })
      setMetaTag('description', site.description)
      setMetaTag('keywords', site.keywords)
      applyFavicon(site.icon)
    } catch (error) {
      console.error('获取站点信息失败:', error)
      toast.error(error instanceof Error ? error.message : '获取站点信息失败：网络异常')
    }
    return readonly(site)
  })()
  return loading
}

/** 系统设置保存站点信息后立即更新标题、描述和图标，不用刷新页面 */
export const applySiteConfig = (patch: Partial<SiteConfig>) => {
  Object.assign(site, patch)
  if (!site.title.trim()) site.title = PRODUCT_NAME
  setMetaTag('description', site.description)
  setMetaTag('keywords', site.keywords)
  applyFavicon(site.icon)
}

export const useSite = () => readonly(site)
