import { hasTemplateError, isRandomName } from './template'

/*
 * 系统设置页的表单模型。字段名与 PATCH /settings 一致；
 * 默认存储和本地 CDN 域名在存储页设置，这里不包含。
 */

export type SettingsGroup = 'upload' | 'image' | 'security' | 'seo'

export const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp'] as const
export const MiB = 1024 * 1024

export interface SettingsDraft {
  allowed_types: string[]
  max_file_size: number
  video_upload_enabled: boolean
  max_video_file_size: number
  default_path: string
  file_name: string
  save_original_name: boolean
  original_image: boolean
  save_webp: boolean
  main_image_quality: number
  /** MIME 或扩展名，保留后端里已有的其他写法 */
  skip_compress_formats: string[]
  thumbnail: boolean
  registration_enabled: boolean
  referer_white_enable: boolean
  referer_white_list: string
  seo_title: string
  seo_description: string
  seo_keywords: string
  seo_icon: string
  seo_icp: string
  public_security: string
}

export type SettingsKey = keyof SettingsDraft

export const FIELD_GROUP: Record<SettingsKey, SettingsGroup> = {
  allowed_types: 'upload',
  max_file_size: 'upload',
  video_upload_enabled: 'upload',
  max_video_file_size: 'upload',
  default_path: 'upload',
  file_name: 'upload',
  save_original_name: 'upload',
  original_image: 'image',
  save_webp: 'image',
  main_image_quality: 'image',
  skip_compress_formats: 'image',
  thumbnail: 'image',
  registration_enabled: 'security',
  referer_white_enable: 'security',
  referer_white_list: 'security',
  seo_title: 'seo',
  seo_description: 'seo',
  seo_keywords: 'seo',
  seo_icon: 'seo',
  seo_icp: 'seo',
  public_security: 'seo',
}

export const FIELD_LABEL: Record<string, string> = {
  allowed_types: '图片类型',
  max_file_size: '图片大小上限',
  video_upload_enabled: '允许上传视频',
  max_video_file_size: '视频大小上限',
  default_path: '存储路径',
  file_name: '文件名',
  save_original_name: '文件名',
  original_image: '保存原图',
  save_webp: '保存为 WebP',
  main_image_quality: '压缩质量',
  skip_compress_formats: '跳过压缩的格式',
  thumbnail: '生成缩略图',
  registration_enabled: '开放用户注册',
  referer_white_enable: '来源白名单',
  referer_white_list: '允许的来源域名',
  seo_title: '网站标题',
  seo_description: '网站描述',
  seo_keywords: '关键词',
  seo_icon: '网站图标',
  seo_icp: 'ICP 备案号',
  public_security: '公安备案号',
  // 不在本页编辑，但后端可能报出
  default_storage: '默认存储',
  cdn_domain: '本地 CDN 域名',
}

export const SETTINGS_KEYS = Object.keys(FIELD_GROUP) as SettingsKey[]

const splitList = (value: unknown) => String(value ?? '').split(',').map(item => item.trim()).filter(Boolean)

/** 服务器返回的设置 → 表单值；只包含响应里有的字段（取决于分组权限） */
export const fromServer = (data: Record<string, unknown>): Partial<SettingsDraft> => {
  const result: Record<string, unknown> = {}
  for (const key of SETTINGS_KEYS) {
    if (!(key in data)) continue
    const value = data[key]
    if (key === 'allowed_types') result[key] = splitList(value).map(item => item.toLowerCase())
    else if (key === 'skip_compress_formats') result[key] = splitList(value)
    else if (typeof value === 'boolean' || typeof value === 'number') result[key] = value
    else result[key] = String(value ?? '')
  }
  return result as Partial<SettingsDraft>
}

/** 表单值 → PATCH 请求里的值 */
export const toServer = <K extends SettingsKey>(key: K, value: SettingsDraft[K]) => Array.isArray(value) ? value.join(',') : value

export const sameValue = (a: unknown, b: unknown) => {
  if (Array.isArray(a) && Array.isArray(b)) return a.length === b.length && a.every((item, index) => item === b[index])
  return a === b
}

/** 客户端检查，规则与后端 services/settings.go 一致；返回 { 字段: 提示 } */
export const validateDraft = (draft: Partial<SettingsDraft>, dirty: (key: SettingsKey) => boolean) => {
  const errors: Partial<Record<SettingsKey, string>> = {}
  if (draft.allowed_types && !draft.allowed_types.length) errors.allowed_types = '至少保留一种图片类型'
  if (draft.max_file_size !== undefined && dirty('max_file_size')) {
    const value = draft.max_file_size
    if (!Number.isFinite(value) || value < 1024 || value > 1024 * MiB) errors.max_file_size = '需要在 1 KB 到 1024 MB 之间'
  }
  if (draft.max_video_file_size !== undefined && dirty('max_video_file_size')) {
    const value = draft.max_video_file_size
    if (!Number.isFinite(value) || value < MiB || value > 1024 * MiB) errors.max_video_file_size = '需要在 1 MB 到 1024 MB 之间'
  }
  if (draft.main_image_quality !== undefined) {
    const value = draft.main_image_quality
    if (!Number.isInteger(value) || value < 0 || value > 100) errors.main_image_quality = '需要是 0–100 的整数'
  }
  if (draft.skip_compress_formats && !draft.skip_compress_formats.length) errors.skip_compress_formats = '至少保留一种格式'
  // 和后端一样只检查改过的模板：升级前保存的旧值不阻止保存其他设置
  if (draft.default_path !== undefined && dirty('default_path') && hasTemplateError(draft.default_path, 'path')) errors.default_path = '存储路径有错误'
  if (draft.file_name !== undefined && dirty('file_name') && !draft.save_original_name && !isRandomName(draft.file_name) && hasTemplateError(draft.file_name, 'name')) errors.file_name = '文件名有错误'
  return errors
}
