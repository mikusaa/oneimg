import { describe, expect, it } from 'vitest'
import type { EventEmitter } from 'node:events'
import { createUploadForm, uploadImages } from '../src/uploader'
import type { PicGoContext, PicGoImage } from '../src/types'

function context(output: PicGoImage[], responses: unknown[]): { ctx: PicGoContext; calls: any[]; notifications: any[] } {
  const calls: any[] = []
  const notifications: any[] = []
  let responseIndex = 0
  const ctx = {
    output,
    request: async (options: unknown) => {
      calls.push(options)
      return responses[responseIndex++]
    },
    getConfig: (key?: string) => key === 'picBed.oneimg'
      ? { _id: 'config-1', serverUrl: 'https://img.example.com', token: 'pat', tagIds: '2,3' }
      : undefined,
    uploaderConfig: {
      getActiveConfig: () => ({ _id: 'config-1', serverUrl: 'https://img.example.com', token: 'pat', tagIds: '2,3' }),
      getConfigList: () => [],
    },
    emit: (event: string, value: unknown) => { if (event === 'notification') notifications.push(value) },
    log: { info: () => undefined, warn: () => undefined, error: () => undefined },
  } as unknown as PicGoContext & EventEmitter
  return { ctx, calls, notifications }
}

function successResponse(ids: number[], start = 0): unknown {
  return {
    statusCode: 200,
    body: {
      data: {
        files: ids.map((id, index) => ({
          file_name: `file-${start + index}.png`,
          success: true,
          duplicate: index === 0,
          image: {
            id,
            url: `/uploads/${id}.webp`,
            bucket_id: 2,
            width: 100,
            height: 80,
            file_size: 42,
            mime_type: 'image/webp',
          },
        })),
      },
    },
  }
}

function image(name: string): PicGoImage {
  return { fileName: name, buffer: Buffer.from(name) }
}

describe('OneImg uploader', () => {
  it('writes repeated multipart fields for images and tag IDs', () => {
    const form = createUploadForm([image('a.png'), image('b.png')], { bucketId: 7, tagIds: [2, 3] })
    const body = form.getBuffer().toString('utf8')
    expect((body.match(/name="images"/g) || []).length).toBe(2)
    expect((body.match(/name="tag_ids"/g) || []).length).toBe(2)
    expect(body).toContain('name="bucket_id"')
    expect(body).toContain('filename="a.png"')

    const withoutBucket = createUploadForm([image('c.png')], { tagIds: [] }).getBuffer().toString('utf8')
    expect(withoutBucket).not.toContain('name="bucket_id"')
  })

  it('uploads one image and keeps the original file name', async () => {
    const output = [image('original-name.png')]
    const { ctx, calls } = context(output, [successResponse([7])])

    await uploadImages(ctx)

    expect(calls).toHaveLength(1)
    expect(output[0].fileName).toBe('original-name.png')
    expect(output[0]).toMatchObject({
      type: 'oneimg',
      imgUrl: 'https://img.example.com/uploads/7.webp',
      width: 100,
      height: 80,
      size: 42,
      contentType: 'image/webp',
    })
  })

  it('uploads 11 images in two batches and stores remote metadata', async () => {
    const output = Array.from({ length: 11 }, (_, index) => image(`${index}.png`))
    const { ctx, calls } = context(output, [successResponse(Array.from({ length: 10 }, (_, i) => i + 1)), successResponse([11], 10)])

    await uploadImages(ctx)

    expect(calls).toHaveLength(2)
    expect(output.every((item) => item.type === 'oneimg')).toBe(true)
    expect(output.map((item) => item.oneimg.imageId)).toEqual(Array.from({ length: 11 }, (_, i) => i + 1))
    expect(output.every((item) => item.imgUrl?.startsWith('https://img.example.com/uploads/'))).toBe(true)
    expect(output.every((item) => item.buffer === undefined)).toBe(true)
    expect(output[0].oneimg.duplicate).toBe(true)
    expect(output[0].oneimg.configId).toBe('config-1')
  })

  it('keeps failed items while allowing partial success', async () => {
    const output = [image('ok.png'), image('bad.png')]
    const response = {
      statusCode: 200,
      body: { data: { files: [
        { file_name: 'ok.png', success: true, image: { id: 10, url: 'https://cdn.example.com/ok.webp', bucket_id: 2 } },
        { file_name: 'bad.png', success: false, error: { code: 'file_upload_failed', detail: '尺寸超限' } },
      ] } },
    }
    const { ctx, notifications } = context(output, [response])

    await uploadImages(ctx)

    expect(output[0].imgUrl).toBe('https://cdn.example.com/ok.webp')
    expect(output[0].buffer).toBeUndefined()
    expect(output[1].buffer).toBeDefined()
    expect(output[1].error).toBeInstanceOf(Error)
    expect(notifications[0].body).toContain('成功 1 张，失败 1 张')
  })

  it('throws when every file fails or the response count is invalid', async () => {
    const allFailed = [image('bad.png')]
    const failedContext = context(allFailed, [{ statusCode: 200, body: { data: { files: [{ file_name: 'bad.png', success: false, error: { detail: '失败' } }] } } }])
    await expect(uploadImages(failedContext.ctx)).rejects.toThrow('失败')

    const mismatch = [image('a.png'), image('b.png')]
    const mismatchContext = context(mismatch, [successResponse([1])])
    await expect(uploadImages(mismatchContext.ctx)).rejects.toThrow('文件数量')
    expect(mismatch[0].buffer).toBeDefined()
  })
})
