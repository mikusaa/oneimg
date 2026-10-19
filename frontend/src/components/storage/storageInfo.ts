import type { StorageBucket } from '@/api/client.ts'

/** 后端存储类型：default 是内置本地存储，其余是可添加的远端存储 */
export type StorageType = 'default' | 's3' | 'r2' | 'ftp' | 'webdav'
export const REMOTE_TYPES: StorageType[] = ['r2', 's3', 'ftp', 'webdav']

export const TYPE_LABELS: Record<string, string> = {
  default: '本地', s3: 'S3', r2: 'R2', ftp: 'FTP', webdav: 'WebDAV',
}
const TYPE_DESCRIPTIONS: Record<string, string> = {
  default: '服务器磁盘', s3: 'S3 兼容存储', r2: 'Cloudflare R2', ftp: 'FTP', webdav: 'WebDAV',
}

/** 只有 S3 / R2 可以设置直链域名（与后端 publicurl.SupportsStorage 一致） */
export const supportsDirectDomain = (type: string) => type === 's3' || type === 'r2'

const text = (value: unknown) => (typeof value === 'string' ? value.trim() : '')

/** 卡片标题下的一行：类型和存储位置，不含任何密钥 */
export const storageLocation = (storage: StorageBucket) => {
  const config = storage.config ?? {}
  const type = storage.type
  let place = ''
  if (type === 'default') place = 'uploads 目录'
  else if (type === 's3' || type === 'r2') {
    const endpoint = text(config[`${type}_endpoint`])
    let host = endpoint
    try { host = new URL(endpoint).host || endpoint } catch { /* 不是完整 URL 时原样显示 */ }
    place = [host, text(config[`${type}_bucket`])].filter(Boolean).join(' / ')
  } else if (type === 'ftp') {
    const port = Number(config.ftp_port) || 21
    place = text(config.ftp_host) + (port !== 21 ? `:${port}` : '')
  } else if (type === 'webdav') place = text(config.webdav_url)
  return [TYPE_DESCRIPTIONS[type] ?? type, place].filter(Boolean).join(' · ')
}

/** 与后端 publicurl.NormalizeDomain 一致：只缺协议时补 https://，去掉末尾 / */
export const normalizeDomain = (value: string) => {
  let domain = value.trim()
  if (!domain) return ''
  if (!domain.includes('://')) domain = `https://${domain}`
  return domain.replace(/\/+$/, '')
}

/** 返回错误说明；空字符串表示格式正确或留空 */
export const domainError = (value: string) => {
  const domain = normalizeDomain(value)
  if (!domain) return ''
  let url: URL
  try { url = new URL(domain) } catch { return '域名格式不正确' }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') return '只支持 http 或 https'
  if (!url.hostname) return '缺少主机名'
  if (url.search || url.hash || /[?#]/.test(domain)) return '不能包含查询参数或锚点'
  return ''
}

export interface StorageAccess {
  /** 浏览器直接访问域名，不经过 OneImg */
  direct: boolean
  domain: string
  /** 本地 CDN 的链接去掉 /uploads 前缀 */
  stripUploads: boolean
}

/** 每个存储的访问方式：本地看 CDN 域名，S3 / R2 看各自的直链域名，FTP / WebDAV 始终经 OneImg */
export const storageAccess = (storage: StorageBucket, cdnDomain: string): StorageAccess => {
  if (storage.type === 'default') {
    const domain = normalizeDomain(cdnDomain)
    return { direct: !!domain, domain, stripUploads: !!domain }
  }
  if (supportsDirectDomain(storage.type)) {
    const domain = normalizeDomain(text(storage.config?.public_domain))
    return { direct: !!domain, domain, stripUploads: false }
  }
  return { direct: false, domain: '', stripUploads: false }
}

const SAMPLE_PATH = '/uploads/2026/10/k3j9x2ab.webp'

/** 链接示例；实际目录取决于系统设置里的存储路径 */
export const sampleUrl = (access: Pick<StorageAccess, 'domain' | 'stripUploads'>) => {
  if (!access.domain) return `${window.location.origin}${SAMPLE_PATH}`
  return access.domain + (access.stripUploads ? SAMPLE_PATH.slice('/uploads'.length) : SAMPLE_PATH)
}
