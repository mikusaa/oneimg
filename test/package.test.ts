import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

interface PluginPackage {
  name?: string
  main?: string
  author?: { name?: string } | string
  keywords?: string[]
}

describe('plugin package metadata', () => {
  it('contains the fields required by PicGo and PicList plugin discovery', () => {
    const pkg = JSON.parse(readFileSync(resolve(process.cwd(), 'package.json'), 'utf8')) as PluginPackage

    expect(pkg.name).toBe('picgo-plugin-oneimg')
    expect(pkg.main).toBe('dist/index.cjs')
    expect(pkg.keywords).toEqual(expect.arrayContaining(['picgo-plugin', 'picgo-gui-plugin']))
    expect(typeof pkg.author === 'string' ? pkg.author : pkg.author?.name).toBeTruthy()
  })
})
