export const SUPER_ADMIN_ID = 1
export const ROLE_ADMIN = 1
export const ROLE_GUEST = 2
export const ROLE_USER = 3

export const getStoredUser = () => {
  try {
    return JSON.parse(localStorage.getItem('userInfo') || '{}')
  } catch {
    return {}
  }
}

export const getPermissionCodes = (user = getStoredUser()) => {
  const permission = user?.permission || user?.Permission || {}
  return permission.codes || permission.Codes || []
}

export const isSuperAdmin = (user = getStoredUser()) => Number(user?.id ?? user?.ID) === SUPER_ADMIN_ID

export const hasPermission = (code, user = getStoredUser()) => {
  if (isSuperAdmin(user)) return true
  if (Number(user?.role ?? user?.Role) !== ROLE_ADMIN) return false
  const codes = getPermissionCodes(user)
  return codes.includes('*') || codes.includes(code)
}

export const hasAnyPermission = (codes, user = getStoredUser()) => codes.some(code => hasPermission(code, user))
