import { readonly, ref } from 'vue'
import { api, type TagUsage } from '@/api/client.ts'

const TAG_COLORS = ['#10a064', '#3b82f6', '#f59e0b', '#ec7598', '#8b5cf6', '#14b8a6', '#ef4444']
const RECENT_KEY = 'recent-tags'
const RECENT_LIMIT = 5
/** 与后端 validateTagName 一致：去掉首尾空格后 1–10 个字符 */
export const TAG_NAME_MAX = 10

/** 标签颜色只在前端按名称计算，后端没有颜色字段 */
export const tagColor = (name: string) => {
  let hash = 0
  for (const char of name) hash = (hash * 31 + char.charCodeAt(0)) | 0
  return TAG_COLORS[Math.abs(hash) % TAG_COLORS.length]
}

/** 标签名称的长度检查，返回错误提示；通过时返回空字符串 */
export const tagNameError = (name: string) => {
  const length = [...name.trim()].length
  if (!length) return '请输入标签名称'
  if (length > TAG_NAME_MAX) return `标签名称最多 ${TAG_NAME_MAX} 个字符`
  return ''
}

// count 是标签在当前用户可见文件中的使用次数：管理员统计全部文件，普通用户只统计自己的文件
const tags = ref<TagUsage[]>([])
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
const saveRecent = () => {
  try { localStorage.setItem(RECENT_KEY, JSON.stringify(recentIds.value)) } catch { /* 不保存也不影响使用 */ }
}

/** 读取全部标签和使用次数；force 时重新请求（文件的标签变化后用来刷新次数） */
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
  saveRecent()
}

/** 新建标签（需要 tag:create）；同名标签由后端拒绝 */
export const createTag = async (name: string) => {
  const tag = await api.createTag(name.trim())
  if (!tags.value.some(item => item.id === tag.id)) tags.value = [...tags.value, { ...tag, count: 0 }]
  return tag
}

/** 重命名标签（需要 tag:update） */
export const renameTag = async (id: number, name: string) => {
  const tag = await api.updateTag(id, name.trim())
  tags.value = tags.value.map(item => item.id === id ? { ...item, name: tag.name } : item)
  return tag
}

/** 删除标签（需要 tag:delete），文件本身不受影响 */
export const deleteTag = async (id: number) => {
  await api.deleteTag(id)
  tags.value = tags.value.filter(item => item.id !== id)
  if (recentIds.value.includes(id)) {
    recentIds.value = recentIds.value.filter(item => item !== id)
    saveRecent()
  }
}

export const tagById = (id: number) => tags.value.find(tag => tag.id === id)

export const useTags = () => ({ tags: readonly(tags), loaded: readonly(loaded), recentIds: readonly(recentIds) })
