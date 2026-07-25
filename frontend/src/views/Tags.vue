<template>
  <div class="page-shell">
    <section class="page-header">
      <div><h1 class="page-title">标签管理</h1></div>
      <div class="stat-tile min-w-[150px] p-4">
        <p class="text-xs text-slate-400">标签总数</p>
        <p class="mt-1 text-lg font-semibold text-slate-900 dark:text-white">{{ tags.length + 1 }}</p>
      </div>
    </section>

    <div v-if="canCreate" class="toolbar-surface flex gap-3">
      <input v-model="newTag" class="input-modern flex-1" maxlength="10" placeholder="新标签名称" @keyup.enter="createTag" />
      <button class="primary-button" :disabled="saving" @click="createTag"><i class="ri-add-line"></i>添加</button>
    </div>

    <div class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <div class="section-card flex items-center justify-between gap-3 p-4">
        <div class="flex items-center gap-2"><i class="ri-bookmark-fill text-primary"></i><span class="font-medium">默认</span></div>
        <span class="text-xs text-slate-400">系统标签</span>
      </div>
      <div v-for="tag in tags" :key="tag.id" class="section-card flex items-center justify-between gap-3 p-4">
        <div class="min-w-0 flex items-center gap-2"><i class="ri-bookmark-line text-primary"></i><span class="truncate font-medium">{{ tag.name }}</span></div>
        <div class="flex shrink-0 gap-1">
          <button v-if="canUpdate" class="icon-button" title="编辑标签" @click="startEdit(tag)"><i class="ri-edit-line"></i></button>
          <button v-if="canDelete" class="icon-button text-red-500" title="删除标签" @click="deleteTag(tag)"><i class="ri-delete-bin-line"></i></button>
        </div>
      </div>
    </div>

    <div v-if="editingTag" class="fixed inset-0 z-[100] flex items-center justify-center bg-slate-950/60 px-4" @click.self="editingTag = null">
      <form class="section-card w-full max-w-md p-6" @submit.prevent="updateTag">
        <div class="mb-5 flex items-center justify-between"><h2 class="text-lg font-semibold">编辑标签</h2><button type="button" class="icon-button" title="关闭" @click="editingTag = null"><i class="ri-close-line"></i></button></div>
        <input v-model="editName" class="input-modern w-full" maxlength="10" autofocus />
        <div class="mt-5 flex justify-end gap-2"><button type="button" class="soft-button" @click="editingTag = null">取消</button><button class="primary-button" :disabled="saving">保存</button></div>
      </form>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import message from '@/utils/message.js'
import { hasPermission } from '@/utils/permissions.js'

const tags = ref([])
const newTag = ref('')
const editingTag = ref(null)
const editName = ref('')
const saving = ref(false)
const canCreate = hasPermission('tag:create')
const canUpdate = hasPermission('tag:update')
const canDelete = hasPermission('tag:delete')

const loadTags = async () => {
  try {
    const response = await fetch('/api/tags')
    const result = await response.json()
    if (!response.ok || result.code !== 200) throw new Error(result.message || '获取标签失败')
    tags.value = result.data?.list || []
  } catch (error) { message.error(error.message || '获取标签失败') }
}

const createTag = async () => {
  const name = newTag.value.trim()
  if (!name) return message.error('标签名称不能为空')
  saving.value = true
  try {
    const response = await fetch('/api/tags', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) })
    const result = await response.json()
    if (!response.ok || result.code !== 200) throw new Error(result.message || '添加标签失败')
    newTag.value = ''
    message.success('标签已添加')
    await loadTags()
  } catch (error) { message.error(error.message || '添加标签失败') } finally { saving.value = false }
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
    const response = await fetch(`/api/tags/${editingTag.value.id}`, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) })
    const result = await response.json()
    if (!response.ok || result.code !== 200) throw new Error(result.message || '更新标签失败')
    editingTag.value = null
    message.success('标签已更新')
    await loadTags()
  } catch (error) { message.error(error.message || '更新标签失败') } finally { saving.value = false }
}

const deleteTag = async tag => {
  if (!window.confirm(`确认删除标签“${tag.name}”吗？`)) return
  try {
    const response = await fetch(`/api/tags/${tag.id}`, { method: 'DELETE' })
    const result = await response.json()
    if (!response.ok || result.code !== 200) throw new Error(result.message || '删除标签失败')
    message.success('标签已删除')
    await loadTags()
  } catch (error) { message.error(error.message || '删除标签失败') }
}

onMounted(loadTags)
</script>
