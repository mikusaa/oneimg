export type MediaKind = 'all' | 'image' | 'video'
export type UploaderRole = 'all' | 'admin' | 'user'
export type GallerySort = 'new' | 'old' | 'big' | 'name'

/** 图库筛选条件，与 /images 的查询参数一一对应 */
export interface GalleryFilter {
  type: MediaKind
  /** 含任一标签 */
  tags: number[]
  untagged: boolean
  storage: number | null
  role: UploaderRole
  q: string
  sort: GallerySort
}

/** /image-storages：当前用户可见媒体实际所在的存储，含已停用的 */
export interface MediaStorage {
  id: number
  name: string
  type: string
  disabled: boolean
}

export const SORT_OPTIONS: { value: GallerySort; label: string; sort: string; order: 'asc' | 'desc' }[] = [
  { value: 'new', label: '最新上传', sort: 'created_at', order: 'desc' },
  { value: 'old', label: '最早上传', sort: 'created_at', order: 'asc' },
  { value: 'big', label: '体积最大', sort: 'file_size', order: 'desc' },
  { value: 'name', label: '文件名', sort: 'filename', order: 'asc' },
]
