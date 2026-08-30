import FormData from 'form-data'
import { OneImgApiError } from './errors'
import { apiBaseForServerUrl } from './config'
import type {
  OneImgMeResponse,
  OneImgProblem,
  OneImgUploadBatchResponse,
  OneImgUploadOptions,
  OneImgConfig,
} from './types'

export type PicGoRequest = (options: Record<string, unknown>) => Promise<unknown>

export class OneImgClient {
  readonly apiBase: string

  constructor(private readonly request: PicGoRequest, private readonly config: OneImgConfig) {
    this.apiBase = apiBaseForServerUrl(config.serverUrl)
  }

  async me(): Promise<OneImgMeResponse> {
    return this.send<OneImgMeResponse>({ method: 'GET', url: `${this.apiBase}/me` })
  }

  async upload(form: FormData): Promise<OneImgUploadBatchResponse> {
    return this.send<OneImgUploadBatchResponse>({
      method: 'POST',
      url: `${this.apiBase}/images`,
      data: form,
      headers: this.headers(form.getHeaders()),
    })
  }

  async uploadOptions(): Promise<OneImgUploadOptions> {
    return this.send<OneImgUploadOptions>({
      method: 'GET',
      url: `${this.apiBase}/upload-options`,
    })
  }

  async deleteImage(imageId: number): Promise<'deleted' | 'missing'> {
    try {
      await this.send<unknown>({ method: 'DELETE', url: `${this.apiBase}/images/${imageId}` })
      return 'deleted'
    } catch (error) {
      const apiError = OneImgApiError.from(error)
      if (apiError.status === 404) return 'missing'
      throw apiError
    }
  }

  private headers(extra: Record<string, string>): Record<string, string> {
    return {
      ...extra,
      Authorization: `Bearer ${this.config.token}`,
    }
  }

  private async send<T>(options: Record<string, unknown>): Promise<T> {
    try {
      const response = await this.request({
        ...options,
        headers: this.headers((options.headers || {}) as Record<string, string>),
        timeout: this.config.timeoutSeconds * 1000,
        resolveWithFullResponse: true,
      }) as Record<string, unknown>
      const status = Number(response?.statusCode ?? response?.status ?? 200)
      const body = (response?.body ?? response?.data ?? response) as T
      if (status < 200 || status >= 300) {
        throw problemError(status, body)
      }
      return body
    } catch (error) {
      throw OneImgApiError.from(error)
    }
  }
}

function problemError(status: number, body: unknown): OneImgApiError {
  const problem = (body && typeof body === 'object' ? body : {}) as OneImgProblem
  return new OneImgApiError(
    status,
    problem.detail || problem.title || 'OneImg 请求失败',
    problem.code || '',
    problem.request_id || '',
  )
}
