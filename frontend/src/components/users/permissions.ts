/** 管理员的功能权限码，分组与后端 models.Permission 一致（20 个） */
export const PERMISSION_GROUPS = [
  {
    name: '用户', icon: 'ri-group-line', items: [
      { code: 'user:list', label: '查看用户' },
      { code: 'user:create', label: '添加用户' },
      { code: 'user:delete', label: '删除用户' },
      { code: 'user:role:update', label: '修改用户名和角色' },
      { code: 'user:permission:update', label: '编辑权限' },
      { code: 'user:password:reset', label: '重置密码' },
      { code: 'user:passkey:reset', label: '撤销 Passkey' },
    ],
  },
  {
    name: '标签', icon: 'ri-price-tag-3-line', items: [
      { code: 'tag:create', label: '新增标签' },
      { code: 'tag:update', label: '编辑标签' },
      { code: 'tag:delete', label: '删除标签' },
    ],
  },
  {
    name: '存储', icon: 'ri-database-2-line', items: [
      { code: 'storage:create', label: '新增存储' },
      { code: 'storage:update', label: '编辑存储' },
      { code: 'storage:delete', label: '删除存储' },
    ],
  },
  {
    name: '图片（他人上传的）', icon: 'ri-image-line', items: [
      { code: 'image:delete', label: '删除图片' },
      { code: 'image:tag:add', label: '添加图片标签' },
      { code: 'image:tag:delete', label: '删除图片标签' },
    ],
  },
  {
    name: '系统设置', icon: 'ri-settings-3-line', items: [
      { code: 'setting:upload', label: '存储与上传' },
      { code: 'setting:image', label: '图片处理' },
      { code: 'setting:security', label: '访问安全' },
      { code: 'setting:seo', label: '站点信息' },
    ],
  },
] as const

export const ALL_CODES: string[] = PERMISSION_GROUPS.flatMap(group => group.items.map(item => item.code))
