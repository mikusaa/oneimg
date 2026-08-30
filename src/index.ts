import type { IPicGoPluginInterface } from 'picgo'
import { configSchema, getActiveConfigRecord, normalizeConfig } from './config'
import { OneImgClient } from './client'
import { errorMessage } from './errors'
import { createRemoveHandler } from './remover'
import { uploadImages } from './uploader'
import { PLUGIN_NAME, UPLOADER_NAME, type PicGoContext } from './types'

const REMOVE_HANDLER_KEY = Symbol.for('picgo-plugin-oneimg.remove-handler')
type RemoveHandler = (files: any[], guiApi?: any) => Promise<void>

function syncRemoveHandler(ctx: PicGoContext, handler: RemoveHandler): void {
  const state = ctx as unknown as Record<PropertyKey, unknown>
  const previous = state[REMOVE_HANDLER_KEY] as RemoveHandler | undefined
  if (previous && previous !== handler) {
    if (typeof ctx.off === 'function') ctx.off('remove', previous)
    else ctx.removeListener?.('remove', previous)
  }
  if (previous !== handler) {
    state[REMOVE_HANDLER_KEY] = handler
    ctx.on('remove', handler)
  }
}

function createGuiMenu(pluginCtx: PicGoContext) {
  return [{
    label: '测试 OneImg 连接',
    async handle(ctx: PicGoContext, guiApi: { showNotification?: (options: { title: string; body: string }) => void }) {
      const runtimeCtx = ctx || pluginCtx
      try {
        const record = getActiveConfigRecord(runtimeCtx)
        const config = normalizeConfig(record)
        const client = new OneImgClient((options) => runtimeCtx.request(options as never), config)
        const [me, options] = await Promise.all([client.me(), client.uploadOptions()])
        const username = me?.data?.username || '未知用户'
        const uploadOptions = options?.data
        const allowedTypes = uploadOptions?.allowed_types?.filter(Boolean).join(', ') || '未限制'
        const buckets = uploadOptions?.storage_buckets || []
        const bucketSummary = buckets.length
          ? buckets.map((bucket) => `${bucket.name} (#${bucket.id})`).join(', ')
          : '无'
        const body = [
          `连接成功：${username}`,
          `单文件限制 ${formatBytes(uploadOptions?.max_file_size)}`,
          `单批最多 ${uploadOptions?.max_files ?? '未设置'} 张`,
          `允许类型 ${allowedTypes}`,
          `默认存储桶 #${uploadOptions?.default_storage ?? '未设置'}`,
          `可用存储桶 ${bucketSummary}`,
        ].join('；')
        guiApi?.showNotification?.({ title: 'OneImg 连接测试', body })
      } catch (error) {
        guiApi?.showNotification?.({ title: 'OneImg 连接失败', body: errorMessage(error) })
      }
    },
  }]
}

function formatBytes(value: number | undefined): string {
  if (!Number.isFinite(value) || (value as number) < 0) return '未设置'
  const units = ['B', 'KB', 'MB', 'GB']
  let amount = value as number
  let unit = 0
  while (amount >= 1024 && unit < units.length - 1) {
    amount /= 1024
    unit += 1
  }
  const rounded = amount >= 10 || unit === 0 ? Math.round(amount) : Math.round(amount * 10) / 10
  return `${rounded} ${units[unit]}`
}

export default function picgoPlugin(ctx: PicGoContext): IPicGoPluginInterface {
  const removeHandler = createRemoveHandler(ctx)
  const register = () => {
    ctx.helper.uploader.register(UPLOADER_NAME, {
      handle: uploadImages,
      config: configSchema,
      name: 'OneImg',
    })
    syncRemoveHandler(ctx, removeHandler)
  }

  return {
    register,
    uploader: UPLOADER_NAME,
    guiMenu: () => createGuiMenu(ctx),
  }
}
