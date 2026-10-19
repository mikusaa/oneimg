/*
 * 存储路径与文件名模板。规则与后端一致：
 * - 变量与替换：utils/images/images.go 的 ReplaceMagicVariables（区分大小写，{moon} 是 {month} 的旧写法）
 * - 保存时校验：utils/images/templates.go 的 ValidatePathTemplate / ValidateNameTemplate
 * - 文件名：buildOutputFileName（除随机名外都加 -随机ID）和 SafeName（最长 80 字节）
 * - 目录：utils/uploads/uploads.go 的 objectDir（忽略 . 和 ..，本地存储始终放在 uploads 下）
 */

export type TemplateKind = 'path' | 'name'

export const TEMPLATE_VARS: Record<string, { label: string; sample: string }> = {
  year: { label: '年', sample: '2026' },
  month: { label: '月', sample: '10' },
  day: { label: '日', sample: '18' },
  hour: { label: '时', sample: '14' },
  minute: { label: '分', sample: '32' },
  second: { label: '秒', sample: '05' },
  role: { label: '角色', sample: 'admin' },
  filename: { label: '原文件名', sample: 'hero-banner' },
  random: { label: '随机', sample: '8b1e4f0a9c2d7e5f3a6b0c9d8e7f1a2b' },
  uuid: { label: 'UUID', sample: '8b1e4f0a9c2d7e5f3a6b0c9d8e7f1a2b' },
  moon: { label: '月（旧写法）', sample: '10' },
}

/** 拼装器菜单里提供的变量 */
export const TEMPLATE_MENU: Record<TemplateKind, string[]> = {
  path: ['year', 'month', 'day', 'hour', 'role'],
  name: ['filename', 'year', 'month', 'day', 'hour', 'minute', 'second'],
}

export const DEFAULT_PATH = '/uploads/{year}/{month}'

export const PATH_PRESETS = [
  { key: 'ym', label: '按年 / 月', value: '/uploads/{year}/{month}', tag: '默认', desc: '' },
  { key: 'ymd', label: '按年 / 月 / 日', value: '/uploads/{year}/{month}/{day}', tag: '', desc: '每天一个目录，上传量大时更好找' },
  { key: 'role', label: '按角色分开', value: '/uploads/{role}/{year}/{month}', tag: '', desc: '管理员和普通用户的文件分开存放' },
  { key: 'flat', label: '不分目录', value: '/uploads', tag: '', desc: '全部放在一个目录，文件多了不便管理' },
]

/** 示例里的随机 ID，后端 NewObjectID 生成 32 位十六进制 */
export const SAMPLE_ID = '3f9c2a7e5b1d4c8fa0e6b2d9c7a1f4e3'

export type TemplateToken = { t: 'sep' } | { t: 'var'; k: string } | { t: 'text'; v: string }

export const tokenize = (value: string): TemplateToken[] => {
  const out: TemplateToken[] = []
  const pattern = /\{([^{}/]*)\}|\//g
  let last = 0
  let match: RegExpExecArray | null
  while ((match = pattern.exec(value))) {
    if (match.index > last) out.push({ t: 'text', v: value.slice(last, match.index) })
    out.push(match[0] === '/' ? { t: 'sep' } : { t: 'var', k: match[1] })
    last = pattern.lastIndex
  }
  if (last < value.length) out.push({ t: 'text', v: value.slice(last) })
  return out
}

export const joinTokens = (tokens: TemplateToken[]) => tokens.map(token => token.t === 'sep' ? '/' : token.t === 'var' ? `{${token.k}}` : token.v).join('')

/** 用示例值替换变量，认不出的变量原样保留 */
export const fillTemplate = (template: string, stem = TEMPLATE_VARS.filename.sample) => template.replace(/\{([^{}/]*)\}/g, (match, key: string) => {
  if (!TEMPLATE_VARS[key]) return match
  if (key === 'filename') return stem
  return TEMPLATE_VARS[key].sample
})

// 与后端 hasControlRune 一致：控制字符和格式字符（如零宽空格）
const CONTROL = /[\u0000-\u001f\u007f-\u009f­؀-؅؜۝܏᠎​-‏‪-‮⁠-⁤⁦-⁯﻿￹-￻]/
const CONTROL_GLOBAL = new RegExp(CONTROL.source, 'g')
const byteLength = (value: string) => new TextEncoder().encode(value).length

export const safeName = (value: string) => {
  let result = ''
  let bytes = 0
  for (const char of value.replace(CONTROL_GLOBAL, '')) {
    const next = '/\\:"\'<>|?*'.includes(char) ? '-' : char
    const size = byteLength(next)
    if (bytes + size > 80) break
    result += next
    bytes += size
  }
  return result.replace(/^[ .]+|[ .]+$/g, '') || 'file'
}

export const isRandomName = (template: string) => ['', '{random}', '{uuid}'].includes(template.trim())

/** 生成的文件名（示例） */
export const outputName = (options: { keepName: boolean; nameTemplate: string }, original: string, ext: string) => {
  const stem = original.replace(/\.[^.]+$/, '')
  if (options.keepName) return `${safeName(stem)}-${SAMPLE_ID}${ext}`
  const template = options.nameTemplate.trim()
  if (isRandomName(template)) return SAMPLE_ID + ext
  return `${safeName(fillTemplate(template, safeName(stem)))}-${SAMPLE_ID}${ext}`
}

/** 文件所在目录（示例）；storageType 为后端的类型，本地存储是 default */
export const objectDir = (dir: string, storageType?: string) => {
  const cleaned = dir.replace(/\\/g, '/').split('/').filter(part => part !== '.' && part !== '..').join('/').replace(/^\/+|\/+$/g, '')
  if (storageType === 'default' && cleaned !== 'uploads' && !cleaned.startsWith('uploads/')) return `uploads/${cleaned}`.replace(/\/$/, '')
  return cleaned
}

const editDistance = (a: string, b: string) => {
  const d = Array.from({ length: a.length + 1 }, (_, i) => [i, ...Array<number>(b.length).fill(0)])
  for (let j = 1; j <= b.length; j++) d[0][j] = j
  for (let i = 1; i <= a.length; i++) {
    for (let j = 1; j <= b.length; j++) {
      d[i][j] = Math.min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1))
    }
  }
  return d[a.length][b.length]
}

const guessVariable = (key: string) => {
  const lower = key.trim().toLowerCase()
  return Object.keys(TEMPLATE_VARS)
    .filter(name => name !== 'moon')
    .map(name => [name, editDistance(lower, name)] as const)
    .filter(([, distance]) => distance <= 2)
    .sort((a, b) => a[1] - b[1])[0]?.[0]
}

export interface TemplateIssue {
  /** error 阻止保存，warn 只提醒，info 是说明 */
  level: 'error' | 'warn' | 'info'
  message: string
  fix?: (value: string) => string
}

export const validateTemplate = (raw: string, kind: TemplateKind): TemplateIssue[] => {
  const value = raw.trim()
  const issues: TemplateIssue[] = []
  const add = (level: TemplateIssue['level'], message: string, fix?: TemplateIssue['fix']) => issues.push({ level, message, fix })
  if (!value) {
    if (kind === 'path') add('info', `留空时使用 ${DEFAULT_PATH}`, () => DEFAULT_PATH)
    return issues
  }
  const where = kind === 'path' ? '路径' : '文件名'
  const tidy = (part: string) => part.replace(/-{2,}/g, '-').replace(/^-+|-+$/g, '')
  // 路径按目录逐段修整，修完为空的目录直接去掉
  const perSegment = (fn: (part: string) => string) => (input: string) => kind === 'path'
    ? input.split('/').map((part, index) => index === 0 && !part ? part : tidy(fn(part))).filter((part, index) => index === 0 || part).join('/')
    : tidy(fn(input))

  if (CONTROL.test(value)) add('error', `${where}包含看不见的控制字符`, input => input.replace(CONTROL_GLOBAL, ''))
  if (/\s/.test(value)) add(kind === 'path' ? 'error' : 'warn', '包含空格，链接里会变成 %20', perSegment(part => part.trim().replace(/\s+/g, '-')))
  const opens = (value.match(/\{/g) || []).length
  const closes = (value.match(/\}/g) || []).length
  if (opens !== closes || /[{}]/.test(value.replace(/\{[^{}/]*\}/g, ''))) {
    add('error', '花括号没有成对，变量要写成 {year} 这样', input => input.replace(/\{([^{}/]*)\}/g, '\u0000$1\u0001').replace(/[{}]/g, '').replace(/\u0000/g, '{').replace(/\u0001/g, '}'))
  }
  const unknown = [...new Set([...value.matchAll(/\{([^{}/]*)\}/g)].map(match => match[1]).filter(key => !TEMPLATE_VARS[key]))]
  for (const key of unknown) {
    const guess = guessVariable(key)
    add('error', `认不出变量 {${key}}${guess ? `，是不是 {${guess}}？变量区分大小写` : `，它会原样写进${where}`}`,
      guess ? input => input.split(`{${key}}`).join(`{${guess}}`) : input => input.split(`{${key}}`).join(''))
  }
  if (value.includes('{moon}')) add('warn', '{moon} 是旧写法，效果和 {month} 一样', input => input.split('{moon}').join('{month}'))
  if (kind === 'path') {
    if (value.includes('\\')) add('error', '目录之间要用 /，不能用 \\', input => input.replace(/\\/g, '/'))
    if (value.split(/[\\/]/).some(part => part === '.' || part === '..')) add('error', '不能包含 . 或 .. 这样的目录', input => input.split('/').filter(part => part !== '.' && part !== '..').join('/'))
    if (/\/{2,}/.test(value)) add('warn', '有连续的 /', input => input.replace(/\/{2,}/g, '/'))
    const odd = [...new Set(value.replace(/\{[^{}/]*\}/g, '').replace(/[A-Za-z0-9_\-/.{}\\\s]/g, '').replace(CONTROL_GLOBAL, ''))]
    if (odd.length) add('warn', `包含「${odd.join(' ')}」，FTP、WebDAV 等存储可能不支持`, perSegment(part => part.replace(/\{[^{}/]*\}|[^A-Za-z0-9_\-.{}\\\s]/g, match => match.length > 1 ? match : '-')))
    if (/\{(random|uuid)\}/.test(value)) add('warn', '{random} / {uuid} 会让每个文件单独占一个目录，不推荐')
    if (value.includes('{filename}')) add('warn', '{filename} 会按原文件名建目录，目录会非常多')
    if (!/^\/?uploads(\/|$)/.test(value)) {
      const local = `/uploads${`/${value.replace(/^\/+/, '')}`.replace(/\/+$/, '')}`
      add('info', `不以 /uploads 开头：本地存储的文件会自动放进 uploads 目录（如 ${local}）；S3、R2 等远端存储按原样使用这个路径`, input => `/uploads/${input.replace(/^\/+/, '')}`)
    }
  } else {
    if (/[\\/]/.test(value)) add('warn', '文件名里的 / 和 \\ 会被换成 -', input => input.replace(/[\\/]/g, '-'))
    const odd = [...new Set(value.replace(/[^:"'<>|?*]/g, ''))]
    if (odd.length) add('warn', `「${odd.join(' ')}」会被换成 -`, input => input.replace(/[:"'<>|?*]/g, '-'))
    if (/\{(random|uuid)\}/.test(value)) add('info', '后面已经会自动加随机 ID，这里的 {random} / {uuid} 可以去掉', input => input.replace(/[-_]?\{(random|uuid)\}/g, '') || '{random}')
    if (byteLength(fillTemplate(value)) > 80) add('warn', '生成的名字超过 80 字节，超出部分会被截掉')
  }
  return issues
}

export const hasTemplateError = (value: string, kind: TemplateKind) => validateTemplate(value, kind).some(issue => issue.level === 'error')

/** 依次套用所有修正（不含说明类） */
export const fixTemplate = (input: string, kind: TemplateKind) => {
  let value = input
  if (!value.trim()) return value
  for (let i = 0; i < 12; i++) {
    const issue = validateTemplate(value, kind).find(item => item.fix && item.level !== 'info')
    if (!issue?.fix) break
    value = issue.fix(value)
  }
  return kind === 'path' ? `/${value.replace(/^\/+|\/+$/g, '')}` : value
}
