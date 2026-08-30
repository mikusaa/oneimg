import FormData from 'form-data'
import { describe, expect, it } from 'vitest'
import { OneImgClient } from '../src/client'
import { OneImgApiError, redactSecrets } from '../src/errors'

const config = {
  serverUrl: 'https://img.example.com',
  token: 'secret-token',
  tagIds: [],
  deleteRemote: true,
  timeoutSeconds: 30,
}

describe('OneImg client', () => {
  it('adds a Bearer token and timeout to every request', async () => {
    const calls: any[] = []
    const client = new OneImgClient(async (options) => {
      calls.push(options)
      return { statusCode: 200, body: { data: { username: 'admin' } } }
    }, config)

    await client.me()

    expect(calls[0].url).toBe('https://img.example.com/api/v1/me')
    expect(calls[0].headers.Authorization).toBe('Bearer secret-token')
    expect(calls[0].timeout).toBe(30000)
  })

  it('accepts multipart form data and converts API errors to structured errors', async () => {
    const form = new FormData()
    form.append('images', Buffer.from('x'), { filename: 'x.png' })
    const calls: any[] = []
    const client = new OneImgClient(async (options) => {
      calls.push(options)
      return { statusCode: 422, body: { detail: '字段无效', code: 'validation_error', request_id: 'req-1' } }
    }, config)

    await expect(client.upload(form)).rejects.toMatchObject({
      status: 422,
      code: 'validation_error',
      requestId: 'req-1',
    })
    expect(calls[0].headers.Authorization).toBe('Bearer secret-token')
    expect(calls[0].headers['content-type']).toContain('multipart/form-data; boundary=')
  })

  it('maps a not-found delete to an idempotent missing result', async () => {
    const client = new OneImgClient(async () => {
      throw { statusCode: 404, response: { body: { detail: '不存在', code: 'image_not_found' } } }
    }, config)
    await expect(client.deleteImage(3)).resolves.toBe('missing')
  })

  it('parses Problem Details from an Axios response.data error', async () => {
    const client = new OneImgClient(async () => {
      throw { response: { status: 403, data: { detail: '删除权限不足', code: 'scope_missing', request_id: 'req-3' } } }
    }, config)

    await expect(client.deleteImage(4)).rejects.toMatchObject({
      status: 403,
      detail: '删除权限不足',
      code: 'scope_missing',
      requestId: 'req-3',
    })
  })

  it('redacts bearer tokens from low-level error messages', () => {
    const error = OneImgApiError.from({
      statusCode: 403,
      message: 'request failed with Authorization: Bearer secret-token',
      response: { data: { request_id: 'req-2' } },
    })
    expect(error.detail).toContain('Bearer [REDACTED]')
    expect(error.requestId).toBe('req-2')
    expect(JSON.stringify(error)).not.toContain('secret-token')
    expect(redactSecrets('Bearer another-secret')).toBe('Bearer [REDACTED]')
  })
})
