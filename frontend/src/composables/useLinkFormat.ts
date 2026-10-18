import { ref, watch } from 'vue'

export type LinkFormat = 'url' | 'markdown' | 'html' | 'bbcode' | 'gif'

/** 统一的链接格式；「动图式 HTML」只对视频有意义，图片按普通 HTML 生成 */
export const LINK_FORMATS: { value: LinkFormat; label: string }[] = [
  { value: 'url', label: 'URL' },
  { value: 'markdown', label: 'Markdown' },
  { value: 'html', label: 'HTML' },
  { value: 'bbcode', label: 'BBCode' },
  { value: 'gif', label: '动图式 HTML' },
]

const STORAGE_KEY = 'link-format'

const read = (): LinkFormat => {
  try {
    const value = localStorage.getItem(STORAGE_KEY)
    return LINK_FORMATS.some(item => item.value === value) ? value as LinkFormat : 'url'
  } catch {
    return 'url'
  }
}

// 上次选择的链接格式在各页面之间共用，并记在浏览器里
const format = ref<LinkFormat>(read())
watch(format, value => {
  try { localStorage.setItem(STORAGE_KEY, value) } catch { /* 隐私模式等情况下不保存 */ }
})

export const useLinkFormat = () => format
