import type { DashboardStats, Image, Problem, StorageBucket, Tag, UploadBatch, User } from './generated/types.gen'
import { client as generatedClient } from './generated/client.gen'

export type { DashboardStats, Image, Problem, StorageBucket, Tag, UploadBatch, User } from './generated/types.gen'

export type Envelope<T> = { data: T; meta?: { pagination?: { page: number; page_size: number; total: number; total_pages: number } } }

export class ApiError extends Error {
  problem: Problem
  constructor(problem: Problem) { super(problem?.detail || 'API request failed'); this.name = 'ApiError'; this.problem = problem }
}

const baseURL = (import.meta.env.VITE_API_BASE_URL || '').replace(/\/$/, '')

function csrfToken(): string | undefined {
  const value = document.cookie.split('; ').find(item => item.startsWith('oneimg-csrf='))?.slice('oneimg-csrf='.length)
  return value ? decodeURIComponent(value) : undefined
}

generatedClient.setConfig({ baseUrl: `${baseURL}/api/v1`, credentials: 'include' })
generatedClient.interceptors.request.use(request => {
  if (['GET', 'HEAD', 'OPTIONS'].includes(request.method.toUpperCase())) return request
  const token = csrfToken()
  if (!token) return request
  const headers = new Headers(request.headers)
  headers.set('X-OneImg-CSRF', token)
  return new Request(request, { headers })
})

export { generatedClient }

export function apiFetch(input: RequestInfo | URL, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers)
  const method = (init.method || 'GET').toUpperCase()
  if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
    const token = csrfToken()
    if (token) headers.set('X-OneImg-CSRF', token)
  }
  let target = typeof input === 'string' ? input : input.toString()
  if (target.startsWith('/api/v1/')) target = `${baseURL}${target}`
  return fetch(target, { ...init, headers, credentials: 'include' })
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const response = await apiFetch(`${baseURL}/api/v1${path}`, { ...init, headers })
  if (response.status === 204) return undefined as T
  const payload = await response.json().catch(() => null)
  if (!response.ok) throw new ApiError(payload as Problem)
  return (payload as Envelope<T>).data
}

export const api = {
  getPublicConfig: () => request<Record<string, unknown>>('/public/config'),
  login: (body: { username: string; password: string }) => request<User>('/auth/login', { method: 'POST', body: JSON.stringify(body) }),
  register: (body: { username: string; password: string }) => request<User>('/auth/register', { method: 'POST', body: JSON.stringify(body) }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  me: () => request<User>('/me'),
  updateMe: (body: Record<string, unknown>) => request<User>('/me', { method: 'PATCH', body: JSON.stringify(body) }),
  listTags: () => request<Tag[]>('/tags'),
  createTag: (name: string) => request<Tag>('/tags', { method: 'POST', body: JSON.stringify({ name }) }),
  updateTag: (id: number, name: string) => request<Tag>(`/tags/${id}`, { method: 'PATCH', body: JSON.stringify({ name }) }),
  deleteTag: (id: number) => request<void>(`/tags/${id}`, { method: 'DELETE' }),
  listImages: (query = '') => request<Image[]>(`/images${query ? `?${query}` : ''}`),
  getImage: (id: number) => request<Image>(`/images/${id}`),
  deleteImage: (id: number) => request<void>(`/images/${id}`, { method: 'DELETE' }),
  uploadImages: (form: FormData, onProgress?: (value: number) => void) => uploadWithProgress<UploadBatch>(`${baseURL}/api/v1/images`, form, onProgress),
  importImage: (body: Record<string, unknown>) => request<Image>('/image-imports', { method: 'POST', body: JSON.stringify(body) }),
  uploadOptions: () => request<Record<string, unknown>>('/upload-options'),
  dashboardStats: () => request<DashboardStats>('/stats/dashboard'),
  imageStats: (period: string) => request<Record<string, unknown>[]>(`/stats/images?period=${encodeURIComponent(period)}`),
  listBuckets: () => request<StorageBucket[]>('/storage-buckets'),
  listUsers: (query = '') => request<User[]>(`/users${query ? `?${query}` : ''}`),
  getSettings: (groups = '') => request<Record<string, unknown>>(`/settings${groups ? `?groups=${encodeURIComponent(groups)}` : ''}`),
  updateSettings: (body: Record<string, unknown>) => request<Record<string, unknown>>('/settings', { method: 'PATCH', body: JSON.stringify(body) }),
}

function uploadWithProgress<T>(url: string, body: FormData, onProgress?: (value: number) => void): Promise<T> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', url)
    xhr.withCredentials = true
    const token = csrfToken()
    if (token) xhr.setRequestHeader('X-OneImg-CSRF', token)
    xhr.upload.onprogress = event => { if (event.lengthComputable) onProgress?.(Math.round(event.loaded / event.total * 100)) }
    xhr.onerror = () => reject(new Error('Network request failed'))
    xhr.onload = () => {
      const payload = JSON.parse(xhr.responseText || 'null') as Envelope<T> | Problem
      if (xhr.status < 200 || xhr.status >= 300) return reject(new ApiError(payload as Problem))
      resolve((payload as Envelope<T>).data)
    }
    xhr.send(body)
  })
}

export default api
