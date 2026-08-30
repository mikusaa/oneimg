import { describe, expect, it } from 'vitest'
import { EventEmitter } from 'node:events'
import { createRemoveHandler } from '../src/remover'
import type { PicGoContext, PicGoImage } from '../src/types'

function context(deleteRemote = true, response: unknown = { statusCode: 204, body: undefined }): { ctx: PicGoContext; calls: any[]; notifications: any[] } {
  const calls: any[] = []
  const notifications: any[] = []
  const emitter = new EventEmitter()
  const ctx = Object.assign(emitter, {
    request: async (options: unknown) => { calls.push(options); return response },
    getConfig: (key?: string) => {
      if (key === 'picgoPlugins.picgo-plugin-oneimg') return true
      if (key === 'picBed.oneimg') return { _id: 'config-1', serverUrl: 'https://img.example.com', token: 'pat', deleteRemote }
      return undefined
    },
    uploaderConfig: {
      getActiveConfig: () => ({ _id: 'config-1', serverUrl: 'https://img.example.com', token: 'pat', deleteRemote }),
      getConfigList: () => [{ _id: 'config-1', serverUrl: 'https://img.example.com', token: 'pat', deleteRemote }],
    },
    log: { info: () => undefined, warn: () => undefined, error: () => undefined },
  }) as unknown as PicGoContext
  ctx.emit = ((event: string, value: unknown) => { if (event === 'notification') notifications.push(value); return true }) as any
  return { ctx, calls, notifications }
}

function file(imageId: number, configId = 'config-1'): PicGoImage {
  return { type: 'oneimg', fileName: 'image.png', imgUrl: 'https://img.example.com/uploads/image.webp', oneimg: { schemaVersion: 1, imageId, apiBase: 'https://img.example.com/api/v1', configId, bucketId: 2, duplicate: false } }
}

describe('OneImg remote deletion', () => {
  it('deletes by stored image ID and treats 404 as already deleted', async () => {
    const first = context(true, { statusCode: 204, body: undefined })
    await createRemoveHandler(first.ctx)([file(12)], { showNotification: (value) => first.notifications.push(value) })
    expect(first.calls[0].method).toBe('DELETE')
    expect(first.calls[0].url).toBe('https://img.example.com/api/v1/images/12')

    const missing = context(true, { statusCode: 404, body: { detail: '不存在', code: 'image_not_found' } })
    await createRemoveHandler(missing.ctx)([file(13)], { showNotification: (value) => missing.notifications.push(value) })
    expect(missing.notifications[0].body).toContain('已不存在 1')
  })

  it('does not call the API when remote deletion is disabled', async () => {
    const { ctx, calls, notifications } = context(false)
    await createRemoveHandler(ctx)([file(12)], { showNotification: (value) => notifications.push(value) })
    expect(calls).toHaveLength(0)
    expect(notifications).toHaveLength(0)
  })

  it('uses the active config only when its API base matches history', async () => {
    const { ctx, calls } = context(true)
    const item = file(20, 'missing-config')
    await createRemoveHandler(ctx)([item])
    expect(calls).toHaveLength(1)

    const other = context(true)
    const mismatched = { ...file(21, 'missing-config'), oneimg: { ...file(21).oneimg, configId: 'missing-config', apiBase: 'https://other.example.com/api/v1' } }
    await createRemoveHandler(other.ctx)([mismatched], { showNotification: (value) => other.notifications.push(value) })
    expect(other.calls).toHaveLength(0)
    expect(other.notifications[0].body).toContain('找不到原始 OneImg 配置')
  })

  it('uses the stored config ID when another OneImg config is active', async () => {
    const { ctx, calls } = context(true)
    ctx.uploaderConfig.getConfigList = () => [
      { _id: 'config-1', _configName: 'Old', _createdAt: 1, _updatedAt: 1, serverUrl: 'https://img.example.com', token: 'old-pat', deleteRemote: true },
      { _id: 'config-2', _configName: 'Current', _createdAt: 1, _updatedAt: 1, serverUrl: 'https://other.example.com', token: 'current-pat', deleteRemote: true },
    ]
    ctx.uploaderConfig.getActiveConfig = () => ({
      _id: 'config-2', _configName: 'Current', _createdAt: 1, _updatedAt: 1,
      serverUrl: 'https://other.example.com', token: 'current-pat', deleteRemote: true,
    })

    await createRemoveHandler(ctx)([file(24, 'config-1')])

    expect(calls[0].url).toBe('https://img.example.com/api/v1/images/24')
    expect(calls[0].headers.Authorization).toBe('Bearer old-pat')
  })

  it('notifies on permission failures without exposing the token', async () => {
    const { ctx, notifications } = context(true, { statusCode: 403, body: { detail: '无权访问', code: 'permission_denied', request_id: 'req-1' } })
    await createRemoveHandler(ctx)([file(22)], { showNotification: (value) => notifications.push(value) })
    expect(notifications[0].body).toContain('权限不足')
    expect(notifications[0].body).not.toContain('pat')
    expect(notifications[0].body).toContain('images:delete')
  })

  it('identifies an invalid deletion token on 401', async () => {
    const { ctx, notifications } = context(true, { statusCode: 401, body: { detail: '认证失败', code: 'unauthorized', request_id: 'req-401' } })
    await createRemoveHandler(ctx)([file(25)], { showNotification: (value) => notifications.push(value) })
    expect(notifications[0].body).toContain('请检查 Token')
  })

  it('reports storage failures with the request ID and logs deletion context', async () => {
    const { ctx, notifications } = context(true, { statusCode: 502, body: { detail: '物理存储删除失败', code: 'storage_delete_failed', request_id: 'req-502' } })
    const warnings: string[] = []
    ctx.log.warn = (...messages: unknown[]) => { warnings.push(messages.map(String).join(' ')) }

    await createRemoveHandler(ctx)([file(23)], { showNotification: (value) => notifications.push(value) })

    expect(notifications[0].body).toContain('req-502')
    expect(warnings[0]).toContain('request_id=req-502')
    expect(warnings[0]).toContain('image_id=23')
  })
})
