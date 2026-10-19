/** 权限抽屉里可以授权的存储 */
export interface UserStorage {
  id: number
  name: string
  type: string
  disabled: boolean
}
