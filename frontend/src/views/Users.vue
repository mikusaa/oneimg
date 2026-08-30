<template>
  <div class="page-shell">
    <PageHeader title="用户管理" description="管理用户角色、权限和存储范围">
      <template #actions>
        <button v-if="canCreateUser" class="primary-button" @click="openCreateModal">
          <i class="ri-add-line"></i>
          新增用户
        </button>
      </template>
    </PageHeader>

    <!-- 工具栏 -->
    <div
      class="toolbar-surface flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
    >
      <div class="relative flex-1">
        <i
          class="ri-search-line absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 text-sm pointer-events-none"
        ></i>
        <input
          v-model="searchInput"
          type="text"
          placeholder="搜索用户名..."
          class="input-modern pl-9 py-2.5 min-h-0 w-full"
          @input="onSearchInput"
        />
        <i
          v-if="searchInput !== debouncedSearch"
          class="ri-loader-2-line absolute right-3.5 top-1/2 -translate-y-1/2 text-slate-400 text-sm animate-spin"
        ></i>
      </div>
      <div class="flex items-center gap-2.5 w-full sm:w-auto">
        <select
          v-model="roleFilter"
          class="input-modern py-2.5 min-h-0 w-full sm:w-[150px]"
          @change="onRoleFilterChange"
        >
          <option value="all">全部角色</option>
          <option value="1">管理员</option>
          <option value="3">普通用户</option>
        </select>
        <div
          class="stat-tile px-3.5 py-2.5 hidden sm:flex items-center gap-2 shrink-0"
        >
          <span class="text-xs text-slate-400 dark:text-slate-500">共</span>
          <span class="text-sm font-semibold text-slate-900 dark:text-white">{{
            total
          }}</span>
          <span class="text-xs text-slate-400 dark:text-slate-500">位用户</span>
        </div>
      </div>
    </div>

    <!-- 移动端统计 -->
    <div class="sm:hidden stat-tile mt-3 px-3.5 py-2.5 flex items-center gap-2">
      <span class="text-xs text-slate-400 dark:text-slate-500">共</span>
      <span class="text-sm font-semibold text-slate-900 dark:text-white">{{
        total
      }}</span>
      <span class="text-xs text-slate-400 dark:text-slate-500">位用户</span>
      <span
        v-if="debouncedSearch"
        class="ml-auto text-xs text-slate-400 truncate"
      >
        搜索:
        <span class="text-slate-700 dark:text-slate-200">{{
          debouncedSearch
        }}</span>
      </span>
    </div>

    <!-- 首次加载 -->
    <div
      v-if="loading && users.length === 0"
      class="flex min-h-40 items-center justify-center"
    >
      <div
        v-if="showLoading"
        class="inline-flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400"
      >
        <i class="ri-loader-4-line animate-spin text-base" aria-hidden="true"></i>
        <span>正在加载用户</span>
      </div>
    </div>

    <!-- 空数据 -->
    <div
      v-else-if="users.length === 0"
      class="section-card flex flex-col items-center justify-center py-20 text-center mt-4"
    >
      <div
        class="flex items-center justify-center size-16 rounded-full bg-slate-100 dark:bg-slate-800 mb-4"
      >
        <i class="ri-user-line text-2xl text-slate-400 dark:text-slate-500"></i>
      </div>
      <h3 class="text-lg font-medium text-slate-800 dark:text-white mb-1">
        暂无用户数据
      </h3>
      <p class="text-sm text-slate-500 dark:text-slate-400">
        {{
          debouncedSearch
            ? "没有找到匹配的用户，试试其他关键词"
            : '点击右上角"新增用户"按钮创建第一个用户'
        }}
      </p>
    </div>

    <!-- 用户卡片列表 -->
    <div
      v-else
      class="grid grid-cols-[repeat(auto-fit,minmax(min(320px,100%),1fr))] gap-6"
    >
      <div
        v-for="user in users"
        :key="user.id"
        class="group relative rounded-lg border border-slate-200/80 bg-white shadow-sm transition-shadow duration-200 hover:shadow-md dark:bg-slate-900"
        @click="closeDropdown"
      >
        <div class="h-full w-full overflow-hidden rounded-lg">
          <!-- 顶部角色标识条 -->
          <div
            class="h-1.5 w-full"
            :class="
              user.role === 1
                ? 'bg-emerald-500'
                : 'bg-slate-300 dark:bg-slate-600'
            "
          ></div>
          <div class="p-4 flex flex-col gap-3 h-full">
            <div class="flex items-start justify-between">
              <div class="flex items-center gap-3 min-w-0 flex-1 pr-2">
                <div
                  class="shrink-0 size-11 rounded-full flex items-center justify-center text-white font-semibold text-sm"
                  :class="getAvatarColor(user.id)"
                >
                  {{ getInitials(user.username) }}
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2 flex-wrap">
                    <h3
                      class="font-semibold text-sm text-slate-900 dark:text-white truncate"
                      :title="user.username"
                    >
                      {{ user.username }}
                    </h3>
                    <span
                      v-if="user.id === SuperAdminID"
                      class="shrink-0 text-[10px] px-1.5 h-4 leading-4 rounded-full bg-amber-500/15 text-amber-700 dark:text-amber-400 border border-amber-500/20"
                    >
                      超管
                    </span>
                  </div>
                  <p class="text-xs text-slate-400 dark:text-slate-500 mt-0.5">
                    ID: {{ user.id }}
                  </p>
                </div>
              </div>

              <!-- 下拉操作 -->
              <div
				v-if="canManageUsers"
                class="relative shrink-0"
                :ref="(el) => setDropdownRef(user.id, el)"
              >
                <button
                  type="button"
                  class="pressable flex h-10 w-10 items-center justify-center rounded-lg text-slate-400 opacity-100 hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 md:group-hover:opacity-100"
                  :aria-label="`${user.username} 的用户操作`"
                  :aria-expanded="activeDropdown === user.id"
                  aria-haspopup="menu"
                  @click.stop="toggleDropdown(user.id)"
                >
                  <i class="ri-more-2-fill text-base"></i>
                </button>
              </div>
            </div>

            <!-- 标签行 -->
            <div class="flex items-center gap-1.5 flex-wrap">
              <span
                class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border"
                :class="
                  user.role === 1
                    ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20'
                    : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border-slate-200/80 dark:border-white/10'
                "
              >
                <i
                  :class="
                    user.role === 1 ? 'ri-shield-star-line' : 'ri-user-line'
                  "
                  class="text-xs"
                ></i>
                {{ user.role === 1 ? "管理员" : "普通用户" }}
              </span>
              <span
                class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 border-slate-200/80 dark:border-white/10"
              >
                <i class="ri-folder-3-line text-xs"></i>
                {{ getUserBucketCount(user) }} 个存储桶
              </span>
			  <span
				class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 border-slate-200/80 dark:border-white/10"
			  >
				<i class="ri-fingerprint-line text-xs"></i>
				{{ user.passkey_count || 0 }} 个 Passkey
			  </span>
			  <span v-if="user.role === RoleAdmin" class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-500/10 dark:text-emerald-300 dark:border-emerald-500/20">
				<i class="ri-shield-keyhole-line text-xs"></i>{{ getUserCodeCount(user) }} 个权限
			  </span>
            </div>

            <!-- 创建时间 -->
            <div
              class="flex items-center gap-1.5 text-xs text-slate-400 dark:text-slate-500 mt-auto pt-1"
            >
              <i class="ri-calendar-line text-xs"></i>
              <span
                >创建于
                {{ formatDate(user.CreatedAt || user.created_at) }}</span
              >
            </div>
          </div>
        </div>
        <div
          v-if="activeDropdown === user.id"
          :data-user-menu="user.id"
          class="absolute right-[20px] top-[55px] z-[60] mt-1 w-44 rounded-lg border border-slate-200/80 bg-white py-1.5 shadow-xl dark:border-white/10 dark:bg-slate-900"
          role="menu"
          @click.stop
        >
			  <button v-if="canUpdateRole"
            class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left"
            role="menuitem"
            @click="openRoleModal(user)"
          >
            <i class="ri-shield-star-line text-base"></i>
            修改角色
          </button>
			  <button v-if="canUpdatePermission"
            class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left"
            role="menuitem"
            @click="openProfileModal(user)"
          >
            <i class="ri-shield-keyhole-line text-base"></i>
            设置权限
          </button>
			  <button v-if="canResetPassword"
            class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left"
            role="menuitem"
            @click="handleResetPassword(user)"
          >
            <i class="ri-key-2-line text-base"></i>
            重置密码
          </button>
			  <button
				v-if="canRevokePasskeys && user.id !== SuperAdminID && user.id !== currentUserID && user.passkey_count > 0"
				class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition text-left"
				role="menuitem"
				@click="openRevokePasskeysModal(user)"
			  >
				<i class="ri-fingerprint-line text-base"></i>
				撤销全部 Passkey
			  </button>
		  <div v-if="canDeleteUser"
            class="my-1.5 border-t border-slate-100 dark:border-white/5"
          ></div>
			  <button v-if="canDeleteUser"
            class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm transition text-left"
            role="menuitem"
            :class="
              user.id === SuperAdminID
                ? 'text-slate-300 dark:text-slate-600 cursor-not-allowed bg-slate-50 dark:bg-slate-800/30'
                : 'text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20'
            "
            :disabled="user.id === SuperAdminID"
            @click="user.id !== SuperAdminID && openDeleteModal(user)"
          >
            <i class="ri-delete-bin-7-line text-base"></i>
            删除用户
          </button>
        </div>
      </div>
    </div>

    <!-- 分页 -->
    <div
      v-if="totalPages > 1"
      class="flex items-center justify-center gap-1.5 mt-10"
    >
      <button
        type="button"
        class="pressable flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800"
        aria-label="上一页"
        :disabled="page <= 1"
        @click="goToPage(page - 1)"
      >
        <i class="ri-arrow-left-s-line text-sm"></i>
      </button>
      <template v-for="p in pageNumbers" :key="p">
        <span
          v-if="p === '...'"
          class="px-1.5 text-slate-400 text-sm select-none"
          >...</span
        >
        <button
          v-else
          type="button"
          class="pressable flex h-10 min-w-10 items-center justify-center rounded-lg border px-2 text-sm font-medium"
          :aria-label="`第 ${p} 页`"
          :aria-current="page === Number(p) ? 'page' : undefined"
          :class="
            page === Number(p)
              ? 'border-slate-900 dark:border-white bg-slate-900 dark:bg-white text-white dark:text-slate-900 shadow-sm'
              : 'border-slate-200 dark:border-white/10 bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800'
          "
          @click="goToPage(Number(p))"
        >
          {{ p }}
        </button>
      </template>
      <button
        type="button"
        class="pressable flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300 dark:hover:bg-slate-800"
        aria-label="下一页"
        :disabled="page >= totalPages"
        @click="goToPage(page + 1)"
      >
        <i class="ri-arrow-right-s-line text-sm"></i>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { apiFetch } from "@/api/client.ts"
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import PopupModal from '@/utils/popupModal.ts'
import message from '@/utils/message.ts'
import { getStoredUser, hasPermission } from '@/utils/permissions.ts'

const SuperAdminID = 1
const RoleAdmin = 1
const RoleUser = 3
const PAGE_LIMIT = 12
const currentUser = getStoredUser()
const canCreateUser = hasPermission('user:create', currentUser)
const canDeleteUser = hasPermission('user:delete', currentUser)
const canUpdateRole = hasPermission('user:role:update', currentUser)
const canUpdatePermission = hasPermission('user:permission:update', currentUser)
const canResetPassword = hasPermission('user:password:reset', currentUser)
const canRevokePasskeys = hasPermission('user:passkey:reset', currentUser)
const currentUserID = Number(currentUser?.id ?? currentUser?.ID)
const canManageUsers = canDeleteUser || canUpdateRole || canUpdatePermission || canResetPassword || canRevokePasskeys

const PERMISSION_GROUPS = [
  { title: '用户管理', items: [
    { code: 'user:list', name: '查看用户' }, { code: 'user:create', name: '添加用户' },
    { code: 'user:delete', name: '删除用户' }, { code: 'user:role:update', name: '修改角色' },
    { code: 'user:permission:update', name: '编辑权限' }, { code: 'user:password:reset', name: '重置密码' },
    { code: 'user:passkey:reset', name: '撤销 Passkey' }
  ]},
  { title: '内容与标签', items: [
    { code: 'tag:create', name: '新增标签' }, { code: 'tag:update', name: '编辑标签' }, { code: 'tag:delete', name: '删除标签' }
  ]},
  { title: '存储管理', items: [
    { code: 'storage:create', name: '新增存储' }, { code: 'storage:update', name: '编辑存储' }, { code: 'storage:delete', name: '删除存储' }
  ]},
  { title: '图片管理', items: [
    { code: 'image:delete', name: '删除图片' }, { code: 'image:tag:add', name: '添加图片标签' }, { code: 'image:tag:delete', name: '删除图片标签' }
  ]},
  { title: '系统设置', items: [
    { code: 'setting:upload', name: '存储与上传' }, { code: 'setting:image', name: '图片处理' },
    { code: 'setting:security', name: '访问安全' },
    { code: 'setting:seo', name: '站点信息' }
  ]}
]

const users = ref([])
const total = ref(0)
const buckets = ref([]);
const totalPages = ref(0)
const page = ref(1)
const loading = ref(true)
const showLoading = ref(false)
const loadingTimer = ref(null)
const searchInput = ref('')
const debouncedSearch = ref('')
const roleFilter = ref('all')
const activeDropdown = ref(null)
const searchTimer = ref(null)
// 存储每个卡片下拉DOM ref
const dropdownRefs = ref(new Map())

const AVATAR_COLORS = [
  'bg-rose-500', 'bg-amber-500', 'bg-emerald-500', 'bg-cyan-500',
  'bg-violet-500', 'bg-pink-500', 'bg-teal-500', 'bg-orange-500',
]

function getAvatarColor(id) {
  return AVATAR_COLORS[id % AVATAR_COLORS.length]
}

function getInitials(name) {
  return name.slice(0, 2).toUpperCase()
}

function escapeHtml(value) {
  return String(value).replace(/[&<>'"]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' })[char])
}

function formatDate(dateStr) {
  if (!dateStr) return '--'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}

function getUserBucketCount(user) {
  return (user.permission?.bucket_ids || []).length
}

function getUserCodeCount(user) {
  return user.permission?.codes?.length || 0
}

const pageNumbers = computed(() => {
  const total = totalPages.value
  const current = page.value
  if (total <= 5) return Array.from({ length: total }, (_, i) => i + 1)
  const pages: Array<number | '...'> = [1]
  if (current > 3) pages.push('...')
  for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
    pages.push(i)
  }
  if (current < total - 2) pages.push('...')
  if (total > 1) pages.push(total)
  return pages
})

function goToPage(p) {
  if (p >= 1 && p <= totalPages.value) {
    page.value = p
  }
}

// 绑定单个下拉ref
function setDropdownRef(userId, el) {
  if (el) dropdownRefs.value.set(userId, el)
  else dropdownRefs.value.delete(userId)
}

async function toggleDropdown(userId) {
  const opening = activeDropdown.value !== userId
  activeDropdown.value = opening ? userId : null
  if (opening) {
    await nextTick()
    document.querySelector<HTMLElement>(`[data-user-menu="${userId}"] [role="menuitem"]`)?.focus()
  }
}

function closeDropdown() {
  activeDropdown.value = null
}

function handleDropdownKeydown(event) {
  if (event.key !== 'Escape' || !activeDropdown.value) return
  const userId = activeDropdown.value
  closeDropdown()
  nextTick(() => {
    const button = dropdownRefs.value.get(userId)?.querySelector('button') as HTMLButtonElement | null
    button?.focus()
  })
}

// 优化外部点击关闭：仅点击空白区域关闭，不干扰下拉内部
function handleClickOutside(e) {
  if (!activeDropdown.value) return
  const targetId = activeDropdown.value
  const dom = dropdownRefs.value.get(targetId)
  if (!dom) {
    closeDropdown()
    return
  }
  if (!dom.contains(e.target)) {
    closeDropdown()
  }
}

function onSearchInput() {
  if (searchTimer.value) clearTimeout(searchTimer.value)
  searchTimer.value = setTimeout(() => {
    debouncedSearch.value = searchInput.value
    page.value = 1
  }, 300)
}

function onRoleFilterChange() {
  page.value = 1
}

async function fetchUsers() {
  loading.value = true
  showLoading.value = false
  if (loadingTimer.value) clearTimeout(loadingTimer.value)
  loadingTimer.value = setTimeout(() => {
    if (loading.value) showLoading.value = true
  }, 180)
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      page_size: String(PAGE_LIMIT),
    })
    if (debouncedSearch.value) params.set('username', debouncedSearch.value)
    if (roleFilter.value !== 'all') params.set('role', roleFilter.value)

    const res = await apiFetch(`/api/v1/users?${params}`, {
      headers: {}
    })
    const result = await res.json()

    if (res.ok && Array.isArray(result.data)) {
      users.value = result.data
      total.value = result.meta?.pagination?.total || 0
      totalPages.value = result.meta?.pagination?.total_pages || 1
    } else {
      message.error(result.detail || '获取用户列表失败')
    }
  } catch (err) {
    console.error('获取用户列表失败:', err)
    message.error('网络错误，请重试')
  } finally {
    if (loadingTimer.value) clearTimeout(loadingTimer.value)
    loadingTimer.value = null
    showLoading.value = false
    loading.value = false
  }
}

watch([page, debouncedSearch, roleFilter], () => {
  fetchUsers()
})

function openCreateModal() {
  const modal = new PopupModal({
    title: '新增用户',
    type: 'form',
    formFields: [
      {
        name: 'username',
        label: '用户名',
        type: 'text',
        placeholder: '请输入用户名（3-50字符）',
        required: true,
      },
      {
        name: 'password',
        label: '密码',
        type: 'password',
        placeholder: '请输入密码（6-100字符）',
        required: true,
      },
      {
        name: 'role',
        label: '角色',
        type: 'select',
        options: [
          { label: '请选择角色', value: '', disabled: true },
          { label: '管理员', value: '1' },
          { label: '普通用户', value: '3' },
        ],
        required: true,
      },
    ],
    formSubmit: async (modal, formData) => {
      if (!formData.username || formData.username.length < 3) {
        message.warning('用户名至少3个字符')
        return
      }
      if (!formData.password || formData.password.length < 6) {
        message.warning('密码至少6个字符')
        return
      }

      try {
        const res = await apiFetch('/api/v1/users', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            username: formData.username,
            password: formData.password,
            role: parseInt(formData.role) || RoleUser,
          }),
        })
        const result = await res.json()

        if (res.ok && result.data) {
          message.success('用户创建成功')
          modal.close()
          fetchUsers()
        } else {
          message.error(result.detail || '创建失败')
        }
      } catch (err) {
        console.error('创建用户失败:', err)
        message.error('网络错误，请重试')
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '创建',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          )
        },
      },
    ],
  })
  modal.open()
}

function openDeleteModal(user) {
  closeDropdown()
  const modal = new PopupModal({
    title: '确认删除用户',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-error-warning-fill text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
			你确定要删除用户 <strong>${escapeHtml(user.username)}</strong> 吗？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            此操作无法撤销，该用户的所有关联数据将会丢失。
          </p>
        </div>
      </div>
    `,
    type: 'confirm',
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          modal.close()
          try {
            const res = await apiFetch(`/api/v1/users/${user.id}`, {
              method: 'DELETE',
              headers: {
              },
            })
            const result = res.status === 204 ? null : await res.json()

            if (res.ok) {
              message.success('用户已删除')
              fetchUsers()
            } else {
              message.error(result?.detail || '删除失败')
            }
          } catch (err) {
            console.error('删除用户失败:', err)
            message.error('网络错误，请重试')
          }
        },
      },
    ],
  })
  modal.open()
}

function openRevokePasskeysModal(user) {
  closeDropdown()
  const modal = new PopupModal({
    title: '撤销全部 Passkey',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-fingerprint-line text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
            确认撤销用户 <strong>${escapeHtml(user.username)}</strong> 的全部 Passkey？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            撤销后，这些设备将无法再用于登录。
          </p>
        </div>
      </div>
    `,
    type: 'confirm',
    buttons: [
      { text: '取消', type: 'default', callback: currentModal => currentModal.close() },
      {
        text: '确认撤销',
        type: 'danger',
        callback: async currentModal => {
          try {
            const response = await apiFetch(`/api/v1/users/${user.id}/passkeys/revoke`, {
              method: 'POST',
              headers: {}
            })
            const data = response.status === 204 ? null : await response.json()
            if (!response.ok) throw new Error(data?.detail || '撤销失败')
            currentModal.close()
            message.success('Passkey 已撤销')
            fetchUsers()
          } catch (error) {
            message.error(error.message || '撤销失败')
          }
        }
      }
    ]
  })
  modal.open()
}

function openProfileModal(user) {
  const bucketOptions = buckets.value
  const selectedIds = [...(user.permission?.bucket_ids || [])].filter(id => bucketOptions.some(item => item.id === id))
  const selectedCodes = [...(user.permission?.codes || [])]

  const renderBucketCards = () => bucketOptions.length === 0
    ? '<div class="w-full rounded-xl border border-dashed border-slate-200 px-4 py-5 text-center text-sm text-slate-400 dark:border-white/10">暂无可配置的存储源</div>'
    : bucketOptions.map(item => {
        const checked = selectedIds.includes(item.id)
        return `<button type="button" data-bucket-id="${item.id}" class="bucket-card rounded-lg border px-3 py-2 text-sm ${checked ? 'border-primary bg-primary/10 text-primary' : 'border-slate-200 dark:border-white/10'}">${checked ? '<i class="ri-checkbox-circle-fill"></i> ' : ''}${escapeHtml(item.name)}</button>`
      }).join('')

  const renderCodeCards = () => PERMISSION_GROUPS.map(group => `
    <div><p class="mb-2 text-xs font-semibold text-slate-400">${group.title}</p><div class="flex flex-wrap gap-2">
      ${group.items.map(item => {
        const checked = selectedCodes.includes(item.code)
        return `<button type="button" data-code="${item.code}" class="code-card rounded-lg border px-2.5 py-1.5 text-xs ${checked ? 'border-emerald-500 bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300' : 'border-slate-200 dark:border-white/10'}">${checked ? '<i class="ri-checkbox-circle-fill"></i> ' : ''}${item.name}</button>`
      }).join('')}
    </div></div>`).join('')

  const modal = new PopupModal({
    title: '设置用户权限',
    width: '720px',
    content: `<div class="max-h-[65vh] space-y-6 overflow-y-auto pr-1">
      <p class="text-sm text-slate-600 dark:text-slate-300">设置用户 <strong>${escapeHtml(user.username)}</strong> 的功能与存储权限。</p>
      ${user.role === RoleAdmin ? `<section><h4 class="mb-3 font-medium">功能权限</h4><div id="codeCardWrap" class="space-y-4 rounded-lg bg-slate-50 p-3 dark:bg-slate-800/50">${renderCodeCards()}</div></section>` : ''}
      <section><h4 class="mb-3 font-medium">可用存储桶</h4><div id="bucketCardWrap" class="flex flex-wrap gap-2">${renderBucketCards()}</div></section>
    </div>`,
    buttons: [
      { text: '取消', type: 'default', callback: () => modal.close() },
      { text: '确认保存', type: 'primary', callback: async () => {
        try {
          const response = await apiFetch(`/api/v1/users/${user.id}/permissions`, {
            method: 'PUT', headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ bucket_ids: selectedIds, codes: user.role === RoleAdmin ? selectedCodes : [] })
          })
          const data = await response.json()
          if (!response.ok || !data.data) throw new Error(data.detail || '更新失败')
          modal.close()
          message.success('权限更新成功')
          fetchUsers()
        } catch (error) { message.error(error.message || '网络请求异常') }
      }}
    ]
  })
  modal.open()

  const bindInteractions = () => {
    const bucketWrap = document.getElementById('bucketCardWrap')
    bucketWrap?.querySelectorAll<HTMLElement>('.bucket-card').forEach(card => {
      card.onclick = () => {
        const id = Number(card.dataset.bucketId)
        const index = selectedIds.indexOf(id)
        if (index >= 0) selectedIds.splice(index, 1); else selectedIds.push(id)
        bucketWrap.innerHTML = renderBucketCards(); bindInteractions()
      }
    })
    const codeWrap = document.getElementById('codeCardWrap')
    codeWrap?.querySelectorAll<HTMLElement>('.code-card').forEach(card => {
      card.onclick = () => {
        const code = card.dataset.code
        if (!code) return
        const index = selectedCodes.indexOf(code)
        if (index >= 0) selectedCodes.splice(index, 1); else selectedCodes.push(code)
        codeWrap.innerHTML = renderCodeCards(); bindInteractions()
      }
    })
  }
  setTimeout(bindInteractions, 80)
}

function openRoleModal(user) {
  closeDropdown()
  const currentRole = String(user.role)

  const modal = new PopupModal({
    title: '修改用户角色',
    content: `
      <div class="py-1">
        <p class="text-sm text-slate-600 dark:text-slate-300 mb-1">
		  修改用户 <strong class="text-slate-900 dark:text-white">${escapeHtml(user.username)}</strong> 的角色
        </p>
        <div class="mt-3">
          <label class="field-label block mb-1.5">选择角色</label>
          <select
            name="newRole"
            class="input-modern w-full py-2.5"
          >
            <option value="1" ${currentRole === '1' ? 'selected' : ''}>管理员</option>
            <option value="3" ${currentRole === '3' ? 'selected' : ''}>普通用户</option>
          </select>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '重置密码',
        type: 'default',
        callback: () => {
          handleResetPassword(user)
        },
      },
      {
        text: '保存',
        type: 'primary',
        callback: async (modal) => {
          const newRoleSelect = modal.content.querySelector('select[name="newRole"]')
          const newRole = parseInt(newRoleSelect?.value || '3')

          try {
            const res = await apiFetch(`/api/v1/users/${user.id}`, {
              method: 'PATCH',
              headers: {
                'Content-Type': 'application/json',
              },
              body: JSON.stringify({ id: user.id, role: newRole }),
            })
            const result = await res.json()

            if (res.ok && result.data) {
              message.success('角色更新成功')
              modal.close()
              fetchUsers()
            } else {
              message.error(result.detail || '更新失败')
            }
          } catch (err) {
            console.error('更新角色失败:', err)
            message.error('网络错误，请重试')
          }
        },
      },
    ],
  })
  modal.open()
}

async function handleResetPassword(user) {
    const modal = new PopupModal({
    title: '重置用户密码',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-error-warning-fill text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
			你确定要重置用户 <strong>${escapeHtml(user.username)}</strong> 的密码吗？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            此操作无法撤销，请谨慎操作。
          </p>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '确定',
        type: 'primary',
        callback: async () => {
          await resetPassword(user)
        },
      }
    ]
    })
    modal.open()
}

const resetPassword = async (user) => { 
  closeDropdown()
  try {
    const res = await apiFetch(`/api/v1/users/${user.id}/password-reset`, {
      method: 'POST',
      headers: {
      },
    })
    const result = await res.json()

    if (res.ok && result.data?.new_password) {
      const newPassword = result.data.new_password
      const modal = new PopupModal({
        title: '密码重置成功',
        content: `
          <div class="py-1">
            <p class="text-sm text-slate-600 dark:text-slate-300 mb-3">
			  用户 <strong class="text-slate-900 dark:text-white">${escapeHtml(user.username)}</strong> 的新密码已生成，请妥善保管。
            </p>
            <div>
              <label class="field-label block mb-1.5">新密码</label>
              <div class="flex items-center gap-2">
                <code class="flex-1 break-all rounded-lg border border-slate-200 bg-slate-50 px-3.5 py-2.5 font-mono text-sm text-slate-900 dark:border-white/10 dark:bg-slate-800 dark:text-white">
                  ${newPassword}
                </code>
                <button
                  id="copy-pwd-btn"
                  class="soft-button min-h-0 py-2.5 px-3 shrink-0"
                  title="复制密码"
                >
                  <i class="ri-file-copy-line text-base"></i>
                </button>
              </div>
            </div>
          </div>
        `,
        buttons: [
          {
            text: '知道了',
            type: 'primary',
            callback: (modal) => modal.close(),
          },
        ],
      })
      modal.open()

      await nextTick()
      const copyBtn = document.getElementById('copy-pwd-btn')
      if (copyBtn) {
        copyBtn.addEventListener('click', async () => {
          try {
            await navigator.clipboard.writeText(newPassword)
            message.success('密码已复制到剪贴板')
            copyBtn.innerHTML = '<i class="ri-check-line text-base text-emerald-500"></i>'
            setTimeout(() => {
              copyBtn.innerHTML = '<i class="ri-file-copy-line text-base"></i>'
            }, 2000)
          } catch {
            message.warning('复制失败，请手动复制')
          }
        })
      }

      fetchUsers()
    } else {
      message.error(result.detail || '重置失败')
    }
  } catch (err) {
    console.error('重置密码失败:', err)
    message.error('网络错误，请重试')
  }
}
// 获取存储列表
const GetBuckets = async () => {
  try {
    const response = await apiFetch('/api/v1/storage-buckets', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      }
    });
    const result = await response.json();
    if (response.ok && Array.isArray(result.data)) {
      buckets.value = result.data;
    } else {
      message.error(result.detail || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    message.error('获取存储列表失败，请稍后重试');
  }
};

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleDropdownKeydown)
  void GetBuckets()
  void fetchUsers()
})

onUnmounted(() => {
  if (searchTimer.value) clearTimeout(searchTimer.value)
  if (loadingTimer.value) clearTimeout(loadingTimer.value)
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleDropdownKeydown)
  dropdownRefs.value.clear()
})
</script>
