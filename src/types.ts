import type { IImgInfo, IPicGo, IPluginConfig } from 'picgo'

export const UPLOADER_NAME = 'oneimg'
export const PLUGIN_NAME = 'picgo-plugin-oneimg'
export const MAX_BATCH_SIZE = 10
export const DEFAULT_TIMEOUT_SECONDS = 120

export type PicGoContext = IPicGo
export type PicGoImage = IImgInfo & Record<string, unknown>
export type PicGoPluginConfig = IPluginConfig

export interface OneImgConfig {
  serverUrl: string
  token: string
  bucketId?: number
  tagIds: number[]
  deleteRemote: boolean
  timeoutSeconds: number
}

export interface OneImgConfigRecord {
  _id?: string
  _configName?: string
  serverUrl?: unknown
  token?: unknown
  bucketId?: unknown
  tagIds?: unknown
  deleteRemote?: unknown
  timeoutSeconds?: unknown
  [key: string]: unknown
}

export interface OneImgHistoryMeta {
  schemaVersion: 1
  imageId: number
  apiBase: string
  configId: string
  bucketId: number
  duplicate: boolean
}

export interface OneImgImage {
  id: number
  url: string
  thumbnail?: string
  file_name?: string
  original_file_name?: string
  original_file_size?: number
  file_size?: number
  mime_type?: string
  width?: number
  height?: number
  storage?: string
  bucket_id?: number
  created_at?: string
}

export interface OneImgUploadFileResult {
  file_name: string
  success: boolean
  duplicate?: boolean
  image?: OneImgImage
  error?: {
    code?: string
    detail?: string
  }
}

export interface OneImgUploadBatchResponse {
  data?: {
    files?: OneImgUploadFileResult[]
    summary?: {
      total?: number
      succeeded?: number
      failed?: number
    }
  }
}

export interface OneImgProblem {
  type?: string
  title?: string
  status?: number
  detail?: string
  code?: string
  request_id?: string
}

export interface OneImgUploadOptions {
  data?: {
    max_file_size?: number
    allowed_types?: string[]
    max_files?: number
    default_storage?: number
    storage_buckets?: Array<{
      id: number
      name: string
      type: string
    }>
  }
}

export interface OneImgMeResponse {
  data?: {
    id?: number
    username?: string
    role?: number
  }
}
