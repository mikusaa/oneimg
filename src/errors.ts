import type { OneImgProblem } from './types'

export class OneImgApiError extends Error {
  readonly status: number
  readonly code: string
  readonly detail: string
  readonly requestId: string

  constructor(status: number, detail: string, code = '', requestId = '') {
    const safeDetail = redactSecrets(detail)
    super(safeDetail)
    this.name = 'OneImgApiError'
    this.status = status
    this.code = code
    this.detail = safeDetail
    this.requestId = requestId
  }

  static from(error: unknown, fallback = 'OneImg 请求失败'): OneImgApiError {
    if (error instanceof OneImgApiError) return error

    const source = (error && typeof error === 'object' ? error : {}) as Record<string, unknown>
    const response = (source.response && typeof source.response === 'object'
      ? source.response
      : {}) as Record<string, unknown>
    const body = (source.body ?? response.body ?? response.data ?? source.data) as unknown
    const problem = (body && typeof body === 'object' ? body : {}) as OneImgProblem
    const status = toNumber(source.statusCode ?? response.statusCode ?? response.status ?? problem.status)
    const detail = typeof problem.detail === 'string'
      ? redactSecrets(problem.detail)
      : typeof source.message === 'string' && source.message
        ? redactSecrets(source.message)
        : fallback
    const code = typeof problem.code === 'string' ? problem.code : ''
    const requestId = typeof problem.request_id === 'string' ? problem.request_id : ''

    return new OneImgApiError(status, detail, code, requestId)
  }
}

function toNumber(value: unknown): number {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

export function errorMessage(error: unknown): string {
  const apiError = OneImgApiError.from(error)
  if (apiError.status === 401) return 'OneImg 认证失败，请检查 Token'
  if (apiError.status === 403) return 'OneImg 权限不足，请确认 Token 包含所需 scope'
  if (apiError.status === 404) return 'OneImg 资源不存在'
  if (apiError.status === 413) return '上传内容超过 OneImg 限制'
  if (apiError.status === 429) return 'OneImg 请求过于频繁，请稍后重试'
  if (apiError.status === 502) return `OneImg 存储操作失败${apiError.requestId ? `（request_id: ${apiError.requestId}）` : ''}`
  return `${apiError.detail}${apiError.requestId ? `（request_id: ${apiError.requestId}）` : ''}`
}

export function asError(error: unknown): Error {
  if (error instanceof OneImgApiError) return new Error(errorMessage(error))
  if (error instanceof Error) return error
  return new Error(errorMessage(error))
}

export function redactSecrets(value: string): string {
  return value
    .replace(/(authorization\s*[:=]\s*)bearer\s+[^\s,;}"]+/gi, '$1Bearer [REDACTED]')
    .replace(/bearer\s+[^\s,;}"]+/gi, 'Bearer [REDACTED]')
}
