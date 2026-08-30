import { defineConfig } from 'tsup'

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['cjs'],
  platform: 'node',
  target: 'node20',
  bundle: true,
  cjsInterop: true,
  clean: true,
  dts: false,
  noExternal: ['form-data'],
  outDir: 'dist',
  outExtension: () => ({ js: '.cjs' }),
  sourcemap: false,
  footer: {
    js: 'if (module.exports && module.exports.default) module.exports = module.exports.default;',
  },
})
