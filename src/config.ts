import {
  DEFAULT_TIMEOUT_SECONDS,
  PLUGIN_NAME,
  UPLOADER_NAME,
  type OneImgConfig,
  type OneImgConfigRecord,
  type PicGoContext,
  type PicGoPluginConfig,
} from './types'

export function normalizeServerUrl(value: unknown): string {
  const raw = String(value ?? '').trim()
  if (!raw) throw new Error('OneImg 地址不能为空')

  let parsed: URL
  try {
    parsed = new URL(raw)
  } catch {
    throw new Error('OneImg 地址必须是有效的 HTTP/HTTPS URL')
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    throw new Error('OneImg 地址仅支持 http 或 https')
  }
  if (parsed.search || parsed.hash) {
    throw new Error('OneImg 地址不能包含查询参数或锚点')
  }

  let pathname = parsed.pathname.replace(/\/+$/, '')
  if (pathname === '/api/v1') pathname = ''
  else if (pathname.endsWith('/api/v1')) pathname = pathname.slice(0, -'/api/v1'.length).replace(/\/+$/, '')
  parsed.pathname = pathname
  return parsed.toString().replace(/\/$/, '')
}

export function apiBaseForServerUrl(serverUrl: string): string {
  return `${normalizeServerUrl(serverUrl)}/api/v1`
}

export function resolveImageUrl(url: string, serverUrl: string): string {
  const value = String(url ?? '').trim()
  if (!value) return ''
  if (/^https?:\/\//i.test(value)) return value
  const base = normalizeServerUrl(serverUrl)
  if (value.startsWith('/')) {
    const parsed = new URL(base)
    return `${parsed.origin}${value}`
  }
  return `${base}/${value.replace(/^\/+/, '')}`
}

export function parseTagIds(value: unknown): number[] {
  const values = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.trim() ? value.split(',') : []
      : value == null || value === ''
        ? []
        : [value]
  const result: number[] = []
  for (const item of values) {
    if (typeof item === 'string' && item.trim() === '') continue
    const number = typeof item === 'number' ? item : Number(String(item).trim())
    if (!Number.isInteger(number) || number <= 0) throw new Error('标签 ID 必须是正整数')
    if (!result.includes(number)) result.push(number)
  }
  return result
}

function parseOptionalPositiveInt(value: unknown, field: string): number | undefined {
  if (value == null || value === '') return undefined
  const number = typeof value === 'number' ? value : Number(String(value).trim())
  if (!Number.isInteger(number) || number <= 0) throw new Error(`${field} 必须是正整数`)
  return number
}

function parseTimeout(value: unknown): number {
  if (value == null || value === '') return DEFAULT_TIMEOUT_SECONDS
  const number = typeof value === 'number' ? value : Number(String(value).trim())
  if (!Number.isInteger(number) || number < 5 || number > 600) {
    throw new Error('超时时间必须是 5 到 600 秒之间的整数')
  }
  return number
}

export function readConfig(raw: unknown): OneImgConfigRecord {
  const source = (raw && typeof raw === 'object' ? raw : {}) as OneImgConfigRecord
  return {
    ...source,
    serverUrl: typeof source.serverUrl === 'string' ? source.serverUrl.trim() : '',
    token: typeof source.token === 'string' ? source.token.trim() : '',
    bucketId: source.bucketId,
    tagIds: source.tagIds,
    deleteRemote: source.deleteRemote,
    timeoutSeconds: source.timeoutSeconds,
  }
}

export function normalizeConfig(raw: unknown, requireCredentials = true): OneImgConfig {
  const source = readConfig(raw)
  const serverUrl = source.serverUrl ? normalizeServerUrl(source.serverUrl) : ''
  const token = String(source.token ?? '').trim()
  if (requireCredentials && !serverUrl) throw new Error('请填写 OneImg 地址')
  if (requireCredentials && !token) throw new Error('请填写 OneImg Token')

  return {
    serverUrl,
    token,
    bucketId: parseOptionalPositiveInt(source.bucketId, '存储桶 ID'),
    tagIds: parseTagIds(source.tagIds),
    deleteRemote: source.deleteRemote === true || source.deleteRemote === 'true',
    timeoutSeconds: parseTimeout(source.timeoutSeconds),
  }
}

export function getActiveConfigRecord(ctx: PicGoContext): OneImgConfigRecord {
  const fromManager = ctx.uploaderConfig?.getActiveConfig?.(UPLOADER_NAME) as OneImgConfigRecord | undefined
  const fromPicBed = ctx.getConfig<OneImgConfigRecord>(`picBed.${UPLOADER_NAME}`)
  return readConfig({ ...(fromPicBed || {}), ...(fromManager || {}) })
}

export function getConfigId(record: OneImgConfigRecord): string {
  const id = typeof record._id === 'string' ? record._id.trim() : ''
  if (!id) throw new Error('当前 OneImg 配置缺少 PicGo 配置 ID，请重新保存该配置')
  return id
}

export function getConfigRecords(ctx: PicGoContext): OneImgConfigRecord[] {
  const records = ctx.uploaderConfig?.getConfigList?.(UPLOADER_NAME) as OneImgConfigRecord[] | undefined
  if (Array.isArray(records) && records.length > 0) return records.map(readConfig)
  return [getActiveConfigRecord(ctx)]
}

export function configSchema(ctx: PicGoContext): PicGoPluginConfig[] {
  const current = readConfig(ctx.getConfig<OneImgConfigRecord>(`picBed.${UPLOADER_NAME}`))
  return [
    {
      name: 'serverUrl', type: 'input', required: true, default: current.serverUrl,
      alias: 'OneImg 地址', message: '例如: https://img.example.com',
    },
    {
      name: 'token', type: 'password', required: true, default: current.token,
      alias: '个人访问 Token', message: '需要 images:write；删除功能还需要 images:delete',
    },
    {
      name: 'bucketId', type: 'input', required: false, default: current.bucketId ?? '',
      alias: '存储桶 ID', message: '留空使用 OneImg 默认存储桶',
    },
    {
      name: 'tagIds', type: 'input', required: false,
      default: Array.isArray(current.tagIds) ? current.tagIds.join(',') : (current.tagIds ?? ''),
      alias: '标签 ID', message: '可选，多个 ID 用英文逗号分隔',
    },
    {
      name: 'deleteRemote', type: 'confirm', required: false,
      default: current.deleteRemote === true, alias: '同步删除远端图片',
      message: '删除 PicGo 历史记录时删除 OneImg 原图',
      confirmText: '开启', cancelText: '关闭',
    },
    {
      name: 'timeoutSeconds', type: 'input', required: false,
      default: current.timeoutSeconds ?? DEFAULT_TIMEOUT_SECONDS,
      alias: '请求超时（秒）', message: '范围 5-600，默认 120',
    },
  ]
}

export function isPluginDisabled(ctx: PicGoContext): boolean {
  return ctx.getConfig<boolean>(`picgoPlugins.${PLUGIN_NAME}`) === false
}
