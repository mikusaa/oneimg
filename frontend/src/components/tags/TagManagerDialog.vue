<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import type { TagUsage } from '@/api/client.ts'
import { toast, UiButton, UiDialog, UiSelect } from '@/components/ui'
import { createTag, deleteTag, renameTag, tagColor, tagNameError, TAG_NAME_MAX, useTags } from '@/composables/useTags'

// 管理标签：搜索、排序、新建、重命名、单个或批量删除；「未使用」配合「全选未使用」用来清理

const props = defineProps<{
  canCreate: boolean
  canUpdate: boolean
  canDelete: boolean
}>()
const emit = defineEmits<{ 'show-tag': [id: number] }>()
const open = defineModel<boolean>('open', { default: false })

const { tags } = useTags()
const state = reactive({
  query: '',
  sort: 'count' as 'count' | 'name' | 'new',
  unused: false,
  selected: [] as number[],
  editing: null as number | null,
  editName: '',
  newName: '',
  confirm: null as number[] | null,
  busy: false,
})
watch(open, value => {
  if (value) Object.assign(state, { query: '', sort: 'count', unused: false, selected: [], editing: null, editName: '', newName: '', confirm: null, busy: false })
})

const unusedCount = computed(() => tags.value.filter(tag => !tag.count).length)
const list = computed(() => {
  const keyword = state.query.trim().toLowerCase()
  let items = [...tags.value]
  if (keyword) items = items.filter(tag => tag.name.toLowerCase().includes(keyword))
  if (state.unused) items = items.filter(tag => !tag.count)
  const compare: Record<string, (a: TagUsage, b: TagUsage) => number> = {
    count: (a, b) => b.count - a.count || a.name.localeCompare(b.name, 'zh'),
    name: (a, b) => a.name.localeCompare(b.name, 'zh'),
    new: (a, b) => b.id - a.id,
  }
  return items.sort(compare[state.sort])
})

const toggleSelected = (id: number) => {
  state.selected = state.selected.includes(id) ? state.selected.filter(item => item !== id) : [...state.selected, id]
}
const toggleUnused = () => {
  state.unused = !state.unused
  state.selected = []
}

const message = (error: unknown, fallback: string) => error instanceof Error && error.message ? error.message : fallback

const create = async () => {
  const name = state.newName.trim()
  const error = tagNameError(name)
  if (error) return toast.error(error)
  state.busy = true
  try {
    await createTag(name)
    state.newName = ''
    toast.success(`已创建标签「${name}」`)
  } catch (err) {
    toast.error(message(err, '创建标签失败'))
  } finally {
    state.busy = false
  }
}

const editInput = ref<HTMLInputElement[] | null>(null)
const startEdit = (tag: TagUsage) => {
  if (!props.canUpdate) return
  state.editing = tag.id
  state.editName = tag.name
  nextTick(() => {
    editInput.value?.[0]?.focus()
    editInput.value?.[0]?.select()
  })
}
const saveEdit = async (tag: TagUsage) => {
  if (state.editing !== tag.id) return
  const name = state.editName.trim()
  if (!name || name === tag.name) { state.editing = null; return }
  const error = tagNameError(name)
  if (error) return toast.error(error)
  state.busy = true
  try {
    await renameTag(tag.id, name)
    state.editing = null
    toast.success('已重命名')
  } catch (err) {
    toast.error(message(err, '重命名失败'))
  } finally {
    state.busy = false
  }
}

const confirmLabel = computed(() => {
  const ids = state.confirm
  if (!ids) return ''
  const name = ids.length === 1 ? `「${tags.value.find(tag => tag.id === ids[0])?.name ?? ''}」` : ` ${ids.length} 个标签`
  const uses = tags.value.filter(tag => ids.includes(tag.id)).reduce((total, tag) => total + tag.count, 0)
  return `删除${name}？${uses ? `相关文件会失去${ids.length === 1 ? '这个标签' : '这些标签'}（共 ${uses} 处），文件本身不受影响。` : '没有文件使用。'}`
})
const remove = async () => {
  const ids = state.confirm
  if (!ids || state.busy) return
  state.busy = true
  let failed = 0
  for (const id of ids) {
    try { await deleteTag(id) } catch { failed += 1 }
  }
  state.busy = false
  state.selected = state.selected.filter(id => !ids.includes(id))
  state.confirm = null
  if (failed) toast.error(`${failed} 个标签删除失败`)
  if (ids.length > failed) toast.success(`已删除 ${ids.length - failed} 个标签`)
}

const showTag = (id: number) => {
  open.value = false
  emit('show-tag', id)
}
</script>

<template>
  <UiDialog v-model:open="open" title="管理标签" :description="`共 ${tags.length} 个标签，${unusedCount} 个未使用。删除标签不会删除文件。`" icon="ri-price-tag-3-line" wide close-button>
    <div class="flex flex-wrap gap-2">
      <label class="relative min-w-[160px] flex-1">
        <i class="ri-search-line pointer-events-none absolute left-[11px] top-1/2 -translate-y-1/2 text-base text-oi-faint" aria-hidden="true" />
        <input v-model="state.query" type="search" placeholder="搜索标签" aria-label="搜索标签" class="tm-input w-full !pl-[34px]">
      </label>
      <UiSelect v-model="state.sort" aria-label="排序">
        <option value="count">使用最多</option>
        <option value="name">按名称</option>
        <option value="new">最新创建</option>
      </UiSelect>
      <UiButton :variant="state.unused ? 'primary' : 'default'" :aria-pressed="state.unused" @click="toggleUnused">
        <i class="ri-ghost-line" aria-hidden="true" />未使用 {{ unusedCount }}
      </UiButton>
    </div>

    <form v-if="canCreate" class="flex h-10 items-center gap-2 rounded-[10px] border border-dashed border-oi-line-2 pl-3 pr-1.5 focus-within:border-solid focus-within:border-oi-accent" @submit.prevent="create">
      <i class="ri-add-line text-oi-muted" aria-hidden="true" />
      <input v-model="state.newName" :maxlength="TAG_NAME_MAX" placeholder="输入名称，回车新建标签" aria-label="新标签名称" class="min-w-0 flex-1 border-0 bg-transparent text-[13.5px] text-oi-ink outline-none placeholder:text-oi-faint">
      <UiButton type="submit" size="sm" variant="primary" :disabled="!state.newName.trim() || state.busy">新建</UiButton>
    </form>

    <ul class="-mx-[22px] max-h-[46vh] min-h-[120px] overflow-auto border-t border-oi-line px-3.5" aria-label="标签">
      <li
        v-for="tag in list"
        :key="tag.id"
        :class="['flex h-11 items-center gap-2.5 border-b border-oi-line px-2', state.selected.includes(tag.id) ? 'bg-oi-accent-soft' : 'hover:bg-oi-panel']"
      >
        <button
          v-if="canDelete"
          type="button"
          role="checkbox"
          :aria-checked="state.selected.includes(tag.id)"
          :aria-label="`选择 ${tag.name}`"
          :class="['grid h-[18px] w-[18px] flex-none place-items-center rounded-md border-[1.5px] text-[13px] transition-colors', state.selected.includes(tag.id) ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']"
          @click="toggleSelected(tag.id)"
        ><i class="ri-check-line" aria-hidden="true" /></button>
        <span class="h-[7px] w-[7px] flex-none rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />
        <input
          v-if="state.editing === tag.id"
          ref="editInput"
          v-model="state.editName"
          :maxlength="TAG_NAME_MAX"
          :aria-label="`重命名 ${tag.name}`"
          class="tm-input h-[30px] min-w-0 flex-1"
          @keydown.enter.prevent="saveEdit(tag)"
          @keydown.esc.stop.prevent="state.editing = null"
          @blur="saveEdit(tag)"
        >
        <span v-else class="min-w-0 flex-1 truncate font-[520] text-oi-ink" @dblclick="startEdit(tag)">{{ tag.name }}</span>
        <button
          type="button"
          class="inline-flex items-center gap-0.5 whitespace-nowrap rounded-md px-2 py-0.5 text-xs text-oi-text enabled:hover:bg-oi-hover enabled:hover:text-oi-accent-2 disabled:text-oi-faint"
          :disabled="!tag.count"
          :title="tag.count ? '在图库中查看' : undefined"
          @click="showTag(tag.id)"
        >
          {{ tag.count ? `${tag.count} 个文件` : '未使用' }}<i v-if="tag.count" class="ri-arrow-right-up-line" aria-hidden="true" />
        </button>
        <UiButton v-if="canUpdate" variant="ghost" size="sm" icon :aria-label="`重命名 ${tag.name}`" title="重命名" @click="startEdit(tag)"><i class="ri-edit-line" aria-hidden="true" /></UiButton>
        <UiButton v-if="canDelete" variant="ghost" size="sm" icon class="!text-oi-danger" :aria-label="`删除 ${tag.name}`" title="删除" @click="state.confirm = [tag.id]"><i class="ri-delete-bin-6-line" aria-hidden="true" /></UiButton>
      </li>
      <li v-if="!list.length" class="py-8 text-center text-[13px] text-oi-muted">{{ tags.length ? '没有匹配的标签' : '还没有标签' }}</li>
    </ul>

    <template #footer>
      <div :class="['flex w-full flex-wrap items-center gap-2 text-[12.5px]', state.confirm ? 'text-oi-danger' : 'text-oi-muted']" role="status">
        <template v-if="state.confirm">
          <span class="flex min-w-0 flex-1 items-center gap-1.5"><i class="ri-error-warning-line flex-none" aria-hidden="true" />{{ confirmLabel }}</span>
          <UiButton size="sm" variant="ghost" @click="state.confirm = null">取消</UiButton>
          <UiButton size="sm" variant="danger-solid" :disabled="state.busy" @click="remove">删除</UiButton>
        </template>
        <template v-else-if="state.selected.length">
          <span class="flex-1">已选 {{ state.selected.length }} 个</span>
          <UiButton size="sm" variant="ghost" @click="state.selected = []">取消选择</UiButton>
          <UiButton size="sm" variant="danger-solid" @click="state.confirm = [...state.selected]"><i class="ri-delete-bin-6-line" aria-hidden="true" />删除所选</UiButton>
        </template>
        <template v-else>
          <span class="flex-1">{{ canUpdate ? '双击名称可以重命名。' : '' }}打开「未使用」可以批量清理。</span>
          <UiButton v-if="state.unused && list.length && canDelete" size="sm" @click="state.selected = list.map(tag => tag.id)">全选未使用</UiButton>
        </template>
      </div>
    </template>
  </UiDialog>
</template>

<style scoped>
.tm-input {
  height: 36px; border-radius: 10px; border: 1px solid var(--oi-line-2); background: var(--oi-raised); padding-inline: 12px;
  font-size: 13.5px; color: var(--oi-ink); outline: none;
}
.tm-input:focus { border-color: var(--oi-accent); box-shadow: 0 0 0 3px var(--oi-ring); }
</style>
