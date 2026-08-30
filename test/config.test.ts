import { describe, expect, it } from 'vitest'
import {
  apiBaseForServerUrl,
  configSchema,
  normalizeConfig,
  normalizeServerUrl,
  parseTagIds,
  resolveImageUrl,
} from '../src/config'
import type { PicGoContext } from '../src/types'

describe('OneImg configuration', () => {
  it('normalizes root and API URLs', () => {
    expect(normalizeServerUrl('https://img.example.com/')).toBe('https://img.example.com')
    expect(normalizeServerUrl('https://img.example.com/api/v1/')).toBe('https://img.example.com')
    expect(normalizeServerUrl('https://img.example.com/oneimg/api/v1')).toBe('https://img.example.com/oneimg')
    expect(apiBaseForServerUrl('https://img.example.com/api/v1')).toBe('https://img.example.com/api/v1')
  })

  it('rejects invalid URL forms', () => {
    expect(() => normalizeServerUrl('')).toThrow('地址不能为空')
    expect(() => normalizeServerUrl('ftp://img.example.com')).toThrow('仅支持 http 或 https')
    expect(() => normalizeServerUrl('https://img.example.com/?token=secret')).toThrow('查询参数')
  })

  it('parses and de-duplicates tag IDs', () => {
    expect(parseTagIds('2, 3,2')).toEqual([2, 3])
    expect(parseTagIds([4, 4, 5])).toEqual([4, 5])
    expect(parseTagIds('')).toEqual([])
    expect(() => parseTagIds('2,nope')).toThrow('正整数')
  })

  it('normalizes optional values and validates required credentials', () => {
    expect(normalizeConfig({
      serverUrl: 'https://img.example.com/api/v1',
      token: ' pat ',
      tagIds: '2,2',
      timeoutSeconds: '60',
      deleteRemote: 'true',
    })).toEqual({
      serverUrl: 'https://img.example.com',
      token: 'pat',
      tagIds: [2],
      deleteRemote: true,
      timeoutSeconds: 60,
      bucketId: undefined,
    })
    expect(() => normalizeConfig({ serverUrl: 'https://img.example.com' })).toThrow('Token')
    expect(() => normalizeConfig({ serverUrl: 'https://img.example.com', token: 'pat', timeoutSeconds: 4 })).toThrow('5 到 600')
  })

  it('resolves relative image URLs without changing absolute URLs', () => {
    expect(resolveImageUrl('/uploads/a.webp', 'https://img.example.com')).toBe('https://img.example.com/uploads/a.webp')
    expect(resolveImageUrl('uploads/a.webp', 'https://img.example.com')).toBe('https://img.example.com/uploads/a.webp')
    expect(resolveImageUrl('https://cdn.example.com/a.webp', 'https://img.example.com')).toBe('https://cdn.example.com/a.webp')
  })

  it('provides localized remote deletion switch states', () => {
    const schema = (deleteRemote?: boolean) => configSchema({
      getConfig: () => deleteRemote === undefined ? undefined : { deleteRemote },
    } as unknown as PicGoContext)
    const remoteDeletion = (deleteRemote?: boolean) => schema(deleteRemote).find((item) => item.name === 'deleteRemote')

    expect(remoteDeletion()).toMatchObject({
      type: 'confirm',
      default: false,
      confirmText: '开启',
      cancelText: '关闭',
    })
    expect(remoteDeletion(true)?.default).toBe(true)
  })
})
