import { describe, expect, it } from 'vitest'
import { EventEmitter } from 'node:events'
import picgoPlugin from '../src/index'
import type { PicGoContext } from '../src/types'

describe('OneImg plugin registration', () => {
  it('keeps one remove listener when registered repeatedly', () => {
    const emitter = new EventEmitter()
    const ctx = Object.assign(emitter, {
      helper: { uploader: { register: () => undefined } },
      getConfig: () => true,
      log: { info: () => undefined, warn: () => undefined, error: () => undefined },
    }) as unknown as PicGoContext

    picgoPlugin(ctx).register(ctx)
    picgoPlugin(ctx).register(ctx)
    expect(emitter.listenerCount('remove')).toBe(1)
  })

  it('removes a listener left by an earlier module load', () => {
    const emitter = new EventEmitter()
    const oldHandler = async () => undefined
    const ctx = Object.assign(emitter, {
      helper: { uploader: { register: () => undefined } },
      getConfig: () => true,
      log: { info: () => undefined, warn: () => undefined, error: () => undefined },
    }) as unknown as PicGoContext
    ;(ctx as unknown as Record<PropertyKey, unknown>)[Symbol.for('picgo-plugin-oneimg.remove-handler')] = oldHandler
    emitter.on('remove', oldHandler)

    picgoPlugin(ctx).register(ctx)

    expect(emitter.listenerCount('remove')).toBe(1)
    expect(emitter.listeners('remove')).not.toContain(oldHandler)
  })
})
