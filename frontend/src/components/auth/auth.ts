/** 用户在系统弹窗里取消、超时或换了设备时不提示错误 */
export const isPasskeyCancellation = (error: unknown) => {
  const value = error as { name?: string; code?: string; cause?: { name?: string } } | null
  const name = value?.cause?.name || value?.name
  return name === 'NotAllowedError' || name === 'AbortError' || value?.code === 'ERROR_CEREMONY_ABORTED'
}

/** 登录后返回的站内地址；只接受以单个 / 开头的路径，避免跳到其他网站 */
export const safeRedirect = (value: unknown) => {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) return '/'
  if (value.startsWith('/login') || value.startsWith('/register')) return '/'
  return value
}

/** 与后端一致：用户名 3–50 个字符，密码 6–100 个字符 */
export const usernameError = (value: string) => {
  const length = [...value.trim()].length
  if (!length) return '请填写用户名'
  return length < 3 || length > 50 ? '用户名需要 3–50 个字符' : ''
}

/** 后端按字节计算密码长度（中文等字符占 3 个字节） */
export const passwordError = (value: string) => {
  if (!value) return '请填写密码'
  const bytes = new TextEncoder().encode(value).length
  return bytes < 6 || bytes > 100 ? '密码需要 6–100 个字符（中文等每字按 3 个计）' : ''
}
