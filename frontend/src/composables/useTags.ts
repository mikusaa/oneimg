import { readonly, ref } from 'vue'
import { api, type Tag } from '@/api/client.ts'

const TAG_COLORS = ['#10a064', '#3b82f6', '#f59e0b', '#ec7598', '#8b5cf6', '#14b8a6', '#ef4444']
const RECENT_KEY = 'recent-tags'
const RECENT_LIMIT = 5
export const TAG_NAME_MAX = 50

/** 标签颜色只在前端按名称计算，后端没有颜色字段 */
export const tagColor = (name: string) => {
  let hash = 0
  for (const char of name) hash = (hash * 31 + char.charCodeAt(0)) | 0
  return TAG_COLORS[Math.abs(hash) % TAG_COLORS.length]
}

const tags = ref<Tag[]>([])
const loaded = ref(false)
let loading: Promise<void> | null = null

const readRecent = (): number[] => {
  try {
    const value = JSON.parse(localStorage.getItem(RECENT_KEY) || '[]')
    return Array.isArray(value) ? value.filter(id => Number.isInteger(id)).slice(0, RECENT_LIMIT) : []
  } catch {
    return []
  }
}
// 「最近使用」只记在当前浏览器里
const recentIds = ref<number[]>(readRecent())

/** 读取当前用户可见的全部标签；force 时重新请求 */
export const loadTags = (force = false) => {
  if (force) loading = null
  loading ??= api.listTags()
    .then(list => {
      tags.value = Array.isArray(list) ? list : []
      loaded.value = true
    })
    .catch(error => {
      loading = null
      throw error
    })
  return loading
}

export const markTagUsed = (id: number) => {
  recentIds.value = [id, ...recentIds.value.filter(item => item !== id)].slice(0, RECENT_LIMIT)
  try { localStorage.setItem(RECENT_KEY, JSON.stringify(recentIds.value)) } catch { /* 不保存也不影响使用 */ }
}

/** 新建标签（需要 tag:create）；同名标签由后端拒绝 */
export const createTag = async (name: string) => {
  const tag = await api.createTag(name)
  if (!tags.value.some(item => item.id === tag.id)) tags.value = [...tags.value, tag]
  return tag
}

export const tagById = (id: number) => tags.value.find(tag => tag.id === id)

export const useTags = () => ({ tags: readonly(tags), loaded: readonly(loaded), recentIds: readonly(recentIds) })
