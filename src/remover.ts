import { OneImgClient } from './client'
import {
  apiBaseForServerUrl,
  getActiveConfigRecord,
  getConfigRecords,
  isPluginDisabled,
  normalizeConfig,
  readConfig,
} from './config'
import { OneImgApiError, errorMessage } from './errors'
import { PLUGIN_NAME, UPLOADER_NAME, type OneImgConfig, type OneImgConfigRecord, type PicGoContext, type PicGoImage } from './types'

export function createRemoveHandler(pluginCtx: PicGoContext) {
  return async function removeHandler(files: PicGoImage[], guiApi?: { showNotification?: (options: { title: string; body: string }) => void }): Promise<void> {
    if (isPluginDisabled(pluginCtx) || !Array.isArray(files)) return

    const targets = files.filter((item) => {
      const meta = item?.oneimg
      return item?.type === UPLOADER_NAME && meta?.schemaVersion === 1 && Number.isInteger(meta.imageId) && meta.imageId > 0
    })
    if (targets.length === 0) return

    let deleted = 0
    let missing = 0
    let skipped = 0
    let unresolved = 0
    let failed = 0
    const failureMessages: string[] = []

    for (const item of targets) {
      const meta = item.oneimg as { schemaVersion: 1; imageId: number; apiBase?: string; configId?: string }
      const resolved = resolveHistoryConfig(pluginCtx, meta)
      if (!resolved) {
        skipped += 1
        unresolved += 1
        failureMessages.push(`图片 ${meta.imageId} 找不到原始 OneImg 配置`)
        continue
      }
      const { config } = resolved
      if (!config.deleteRemote) {
        skipped += 1
        continue
      }

      try {
        const client = new OneImgClient((options) => pluginCtx.request(options as never), config)
        const result = await client.deleteImage(meta.imageId)
        if (result === 'missing') missing += 1
        else deleted += 1
      } catch (error) {
        failed += 1
        const apiError = OneImgApiError.from(error)
        const detail = deleteErrorMessage(apiError)
        failureMessages.push(`图片 ${meta.imageId}: ${detail}`)
        pluginCtx.log.warn(`[${PLUGIN_NAME}] 删除 OneImg 图片失败：${config.serverUrl} image_id=${meta.imageId} status=${apiError.status || 0} code=${apiError.code || '-'} request_id=${apiError.requestId || '-'}`)
      }
    }

    if (deleted === 0 && missing === 0 && unresolved === 0 && failed === 0) return
    const summary = `OneImg 删除完成：已删除 ${deleted}，已不存在 ${missing}，跳过 ${skipped}，失败 ${failed}${failureMessages.length ? `。${failureMessages.join('；')}` : ''}`
    if (failed > 0 || skipped > 0) pluginCtx.log.warn(summary)
    else pluginCtx.log.info(summary)
    notify(guiApi, 'OneImg 删除结果', summary)
  }
}

function deleteErrorMessage(error: OneImgApiError): string {
  if (error.status === 401) return 'OneImg 删除认证失败，请检查 Token'
  if (error.status === 403) return 'OneImg 删除权限不足，请确认 Token 包含 images:delete scope'
  return errorMessage(error)
}

function resolveHistoryConfig(ctx: PicGoContext, meta: { apiBase?: string; configId?: string }): { config: OneImgConfig; record: OneImgConfigRecord } | undefined {
  const records = getConfigRecords(ctx)
  const configId = typeof meta.configId === 'string' ? meta.configId : ''
  const exact = records.find((record) => typeof record._id === 'string' && record._id === configId)
  if (exact) return normalizeRecord(exact)

  const active = getActiveConfigRecord(ctx)
  const activeResult = normalizeRecord(active)
  if (!activeResult) return undefined
  if (meta.apiBase && apiBaseForServerUrl(activeResult.config.serverUrl) === meta.apiBase) return activeResult
  return undefined
}

function normalizeRecord(record: OneImgConfigRecord): { config: OneImgConfig; record: OneImgConfigRecord } | undefined {
  try {
    const config = normalizeConfig(readConfig(record))
    return { config, record }
  } catch {
    return undefined
  }
}

function notify(guiApi: { showNotification?: (options: { title: string; body: string }) => void } | undefined, title: string, body: string): void {
  guiApi?.showNotification?.({ title, body })
}
