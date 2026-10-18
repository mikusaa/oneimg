export const SUPER_ADMIN_ID = 1
export const ROLE_ADMIN = 1
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

/** 与后端 CanAccessImage 一致：超级管理员的文件只有本人能管理，他人的文件需要对应权限码 */
export const canManageImage = (image, permission, user = getStoredUser()) => {
  if (!image) return false
  if (isSuperAdmin(user)) return true
  if (Number(image.user_id) === SUPER_ADMIN_ID) return false
  if (Number(image.user_id) === Number(user?.id)) return true
  return hasPermission(permission, user)
}
