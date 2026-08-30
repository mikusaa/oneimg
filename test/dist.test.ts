import { createRequire } from 'node:module'
import { EventEmitter } from 'node:events'
import { copyFileSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import type { PicGoContext } from '../src/types'

const require = createRequire(`${process.cwd()}/package.json`)

describe('built PicGo plugin', () => {
  it('loads as a callable CommonJS plugin without runtime dependencies', () => {
    const tempDir = mkdtempSync(resolve(tmpdir(), 'picgo-plugin-oneimg-'))
    const copiedBundle = resolve(tempDir, 'index.cjs')
    copyFileSync(resolve(process.cwd(), 'dist/index.cjs'), copiedBundle)

    try {
      const plugin = require(copiedBundle) as (ctx: PicGoContext) => { register: (ctx: PicGoContext) => void }
      expect(typeof plugin).toBe('function')

      const registrations: string[] = []
      const ctx = Object.assign(new EventEmitter(), {
        helper: { uploader: { register: (name: string) => registrations.push(name) } },
        getConfig: () => undefined,
        log: { info: () => undefined, warn: () => undefined, error: () => undefined },
      }) as unknown as PicGoContext

      plugin(ctx).register(ctx)
      expect(registrations).toEqual(['oneimg'])
    } finally {
      rmSync(tempDir, { recursive: true, force: true })
    }
  })
})
