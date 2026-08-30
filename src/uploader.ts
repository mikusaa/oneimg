import FormData from 'form-data'
import { OneImgClient } from './client'
import {
  apiBaseForServerUrl,
  getActiveConfigRecord,
  getConfigId,
  normalizeConfig,
  resolveImageUrl,
} from './config'
import { asError, errorMessage, OneImgApiError } from './errors'
import {
  MAX_BATCH_SIZE,
  UPLOADER_NAME,
  type OneImgHistoryMeta,
  type OneImgUploadBatchResponse,
  type PicGoContext,
  type PicGoImage,
} from './types'

export async function uploadImages(ctx: PicGoContext): Promise<PicGoContext> {
  if (!Array.isArray(ctx.output) || ctx.output.length === 0) return ctx

  const record = getActiveConfigRecord(ctx)
  const config = normalizeConfig(record)
  const client = new OneImgClient((options) => ctx.request(options as never), config)
  const configId = getConfigId(record)
  let successCount = 0
  let failureCount = 0
  let firstError: Error | undefined

  for (let start = 0; start < ctx.output.length; start += MAX_BATCH_SIZE) {
    const batch = ctx.output.slice(start, start + MAX_BATCH_SIZE) as PicGoImage[]
    let response: OneImgUploadBatchResponse
    try {
      const form = createUploadForm(batch, config)
      response = await client.upload(form)
    } catch (error) {
      const normalized = asError(error)
      firstError ||= normalized
      for (const item of batch) {
        item.error = normalized
        failureCount += 1
      }
      continue
    }

    const files = response?.data?.files
    if (!Array.isArray(files) || files.length !== batch.length) {
      const normalized = new Error('OneImg 响应中的文件数量与请求不一致')
      firstError ||= normalized
      for (const item of batch) {
        item.error = normalized
        failureCount += 1
      }
      continue
    }

    files.forEach((result, index) => {
      const item = batch[index]
      if (!result.success || !result.image) {
        const detail = result.error?.detail || result.error?.code || 'OneImg 未返回成功图片'
        const normalized = new Error(detail)
        item.error = normalized
        firstError ||= normalized
        failureCount += 1
        return
      }

      try {
        applyUploadResult(item, result.image, Boolean(result.duplicate), config.serverUrl, configId)
        successCount += 1
      } catch (error) {
        const normalized = asError(error)
        item.error = normalized
        firstError ||= normalized
        failureCount += 1
      }
    })
  }

  if (failureCount > 0) {
    ctx.log.warn(`OneImg 上传完成：成功 ${successCount}，失败 ${failureCount}`)
    emitNotification(ctx, 'OneImg 上传完成', `成功 ${successCount} 张，失败 ${failureCount} 张`)
  } else {
    ctx.log.info(`OneImg 上传成功：${successCount} 张`)
  }
  if (successCount === 0 && firstError) throw firstError
  return ctx
}

export function createUploadForm(items: PicGoImage[], config: { bucketId?: number; tagIds: number[] }): FormData {
  const form = new FormData()
  items.forEach((item, index) => {
    const buffer = imageBuffer(item)
    const fileName = typeof item.fileName === 'string' && item.fileName.trim()
      ? item.fileName
      : `image-${index + 1}`
    form.append('images', buffer, {
      filename: fileName,
      contentType: item.contentType || item.mimeType || 'application/octet-stream',
    })
  })
  for (const tagId of config.tagIds) form.append('tag_ids', String(tagId))
  if (config.bucketId !== undefined) form.append('bucket_id', String(config.bucketId))
  return form
}

function imageBuffer(item: PicGoImage): Buffer {
  if (Buffer.isBuffer(item.buffer)) return item.buffer
  if (typeof item.base64Image === 'string' && item.base64Image) {
    return Buffer.from(item.base64Image.replace(/^data:[^;]+;base64,/, ''), 'base64')
  }
  throw new Error(`图片 ${item.fileName || '未命名文件'} 缺少可上传的数据`)
}

export function applyUploadResult(
  item: PicGoImage,
  image: { id?: number; url?: string; width?: number; height?: number; file_size?: number; mime_type?: string; bucket_id?: number },
  duplicate: boolean,
  serverUrl: string,
  configId: string,
): void {
  if (!Number.isInteger(image.id) || (image.id as number) <= 0) throw new Error('OneImg 响应缺少有效图片 ID')
  if (typeof image.url !== 'string' || !image.url.trim()) throw new Error('OneImg 响应缺少图片 URL')
  if (!Number.isInteger(image.bucket_id) || (image.bucket_id as number) <= 0) throw new Error('OneImg 响应缺少有效存储桶 ID')
  const imgUrl = resolveImageUrl(image.url, serverUrl)
  const bucketId = image.bucket_id as number
  const history: OneImgHistoryMeta = {
    schemaVersion: 1,
    imageId: image.id as number,
    apiBase: apiBaseForServerUrl(serverUrl),
    configId,
    bucketId,
    duplicate,
  }

  item.type = UPLOADER_NAME
  item.imgUrl = imgUrl
  item.url = imgUrl
  item.oneimg = history
  if (typeof image.width === 'number') item.width = image.width
  if (typeof image.height === 'number') item.height = image.height
  if (typeof image.file_size === 'number') item.size = image.file_size
  if (typeof image.mime_type === 'string') {
    item.contentType = image.mime_type
    item.mimeType = image.mime_type
  }
  delete item.buffer
  delete item.base64Image
}

function emitNotification(ctx: PicGoContext, title: string, body: string): void {
  ctx.emit('notification', { title, body })
}
