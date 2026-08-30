<template>
  <section class="space-y-3" aria-labelledby="tag-manager-title">
    <div class="toolbar-surface flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <div class="relative min-w-0 w-full lg:max-w-sm">
        <i class="ri-search-line pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"></i>
        <input
          v-model.trim="searchKeyword"
          type="search"
          class="input-modern w-full pl-9"
          placeholder="搜索标签"
          aria-label="搜索标签"
        >
      </div>
      <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
        <form v-if="canCreate" class="flex min-w-0 gap-2" @submit.prevent="createTag">
          <input
            v-model="newTag"
            class="input-modern min-w-0 flex-1 sm:w-52"
            maxlength="10"
            placeholder="新标签名称"
            aria-label="新标签名称"
          >
          <button class="primary-button shrink-0" :disabled="saving" type="submit">
            <i :class="saving ? 'ri-loader-4-line animate-spin' : 'ri-add-line'"></i>
            添加
          </button>
        </form>
        <span class="shrink-0 text-sm text-slate-500 dark:text-slate-400">{{ visibleTagCount }} 个标签</span>
      </div>
    </div>

    <div class="page-surface p-3.5 sm:p-4">
      <h2 id="tag-manager-title" class="sr-only">标签列表</h2>
      <div v-if="visibleTagCount > 0" class="flex flex-wrap gap-2.5">
        <div v-if="showDefaultTag" class="tag-manage-chip tag-manage-chip-default">
          <i class="ri-bookmark-fill text-primary"></i>
          <span class="max-w-40 truncate">默认</span>
          <span class="tag-manage-chip-note">系统</span>
        </div>
        <div v-for="tag in filteredTags" :key="tag.id" class="tag-manage-chip group">
          <i class="ri-bookmark-line text-primary"></i>
          <span class="max-w-44 truncate" :title="tag.name">{{ tag.name }}</span>
          <span v-if="canUpdate || canDelete" class="tag-manage-chip-actions">
            <button v-if="canUpdate" type="button" class="tag-manage-chip-button" title="编辑标签" :aria-label="`编辑标签 ${tag.name}`" @click="startEdit(tag)">
              <i class="ri-edit-line"></i>
            </button>
            <button v-if="canDelete" type="button" class="tag-manage-chip-button text-red-500 dark:text-red-300" title="删除标签" :aria-label="`删除标签 ${tag.name}`" @click="deleteTag(tag)">
              <i class="ri-delete-bin-line"></i>
            </button>
          </span>
        </div>
      </div>
      <div v-if="visibleTagCount === 0 && searchKeyword" class="px-4 py-12 text-center text-sm text-slate-500 dark:text-slate-400">
        没有找到匹配的标签
      </div>
    </div>

    <AppDialog :model-value="!!editingTag" title="编辑标签" width-class="max-w-md" @update:model-value="value => { if (!value) editingTag = null }">
      <form id="tag-edit-form" class="space-y-4" @submit.prevent="updateTag">
        <label class="field-label" for="tag-edit-name">标签名称</label>
        <input id="tag-edit-name" v-model="editName" class="input-modern w-full" maxlength="10" autofocus>
      </form>
      <template #footer>
        <button type="button" class="soft-button" @click="editingTag = null">取消</button>
        <button type="submit" form="tag-edit-form" class="primary-button" :disabled="saving">
          <i v-if="saving" class="ri-loader-4-line animate-spin"></i>
          保存
        </button>
      </template>
    </AppDialog>
  </section>
</template>

<script setup lang="ts">
import { apiFetch } from "@/api/client.ts"
import { computed, onMounted, ref } from 'vue'
import AppDialog from '@/components/AppDialog.vue'
import message from '@/utils/message.ts'
import PopupModal from '@/utils/popupModal.ts'
import { hasPermission } from '@/utils/permissions.ts'

const emit = defineEmits(['changed'])
const tags = ref([])
const newTag = ref('')
const searchKeyword = ref('')
const editingTag = ref(null)
const editName = ref('')
const saving = ref(false)
const canCreate = hasPermission('tag:create')
const canUpdate = hasPermission('tag:update')
const canDelete = hasPermission('tag:delete')
const filteredTags = computed(() => {
  const keyword = searchKeyword.value.toLocaleLowerCase()
  if (!keyword) return tags.value
  return tags.value.filter(tag => tag.name.toLocaleLowerCase().includes(keyword))
})
const showDefaultTag = computed(() => !searchKeyword.value || '默认'.includes(searchKeyword.value.toLocaleLowerCase()))
const visibleTagCount = computed(() => filteredTags.value.length + (showDefaultTag.value ? 1 : 0))

const loadTags = async () => {
  try {
    const response = await apiFetch('/api/v1/tags')
    const result = await response.json()
    if (!response.ok || !Array.isArray(result.data)) throw new Error(result.detail || '获取标签失败')
    tags.value = result.data
  } catch (error) {
    message.error(error.message || '获取标签失败')
  }
}

const createTag = async () => {
  const name = newTag.value.trim()
  if (!name) return message.error('标签名称不能为空')
  saving.value = true
  try {
    const response = await apiFetch('/api/v1/tags', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) })
    const result = await response.json()
    if (!response.ok || !result.data) throw new Error(result.detail || '添加标签失败')
    newTag.value = ''
    message.success('标签已添加')
    await loadTags()
    emit('changed')
  } catch (error) {
    message.error(error.message || '添加标签失败')
  } finally {
    saving.value = false
  }
}

const startEdit = tag => {
  editingTag.value = tag
  editName.value = tag.name
}

const updateTag = async () => {
  const name = editName.value.trim()
  if (!name) return message.error('标签名称不能为空')
  saving.value = true
  try {
    const response = await apiFetch(`/api/v1/tags/${editingTag.value.id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) })
    const result = await response.json()
    if (!response.ok || !result.data) throw new Error(result.detail || '更新标签失败')
    editingTag.value = null
    message.success('标签已更新')
    await loadTags()
    emit('changed')
  } catch (error) {
    message.error(error.message || '更新标签失败')
  } finally {
    saving.value = false
  }
}

const deleteTag = tag => {
  const modal = new PopupModal({
    title: '删除标签',
    content: '<p class="text-sm text-slate-700 dark:text-slate-200">确认删除这个标签吗？此操作无法撤销。</p>',
    buttons: [
      { text: '取消', type: 'default', callback: current => current.close() },
      { text: '删除', type: 'danger', callback: async current => {
        current.close()
        try {
          const response = await apiFetch(`/api/v1/tags/${tag.id}`, { method: 'DELETE' })
          const result = response.status === 204 ? null : await response.json()
          if (!response.ok) throw new Error(result.detail || '删除标签失败')
          message.success('标签已删除')
          await loadTags()
          emit('changed')
        } catch (error) {
          message.error(error.message || '删除标签失败')
        }
      } },
    ],
  })
  modal.open()
}

onMounted(loadTags)
</script>
