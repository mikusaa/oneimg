<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TagUsage } from '@/api/client.ts'
import StorageTypeIcon from '@/components/storage/StorageTypeIcon.vue'
import { tagById, tagColor, useTags } from '@/composables/useTags'
import type { GalleryFilter, MediaKind, MediaStorage, UploaderRole } from './types'

// 图库左侧的筛选栏；手机端放在抽屉里，内容相同

const props = defineProps<{
  storages: MediaStorage[]
  isAdmin: boolean
  canManageTags: boolean
}>()
const emit = defineEmits<{ 'manage-tags': [] }>()
const filter = defineModel<GalleryFilter>({ required: true })

const { tags } = useTags()
const TOP_TAGS = 8
const tagQuery = ref('')
const showAll = ref(false)

const byUsage = (a: TagUsage, b: TagUsage) => b.count - a.count || a.name.localeCompare(b.name, 'zh')
// 默认只显示最常用的 8 个；已选中的标签始终显示
const railTags = computed(() => {
  const keyword = tagQuery.value.trim().toLowerCase()
  const sorted = [...tags.value].sort(byUsage)
  if (keyword) return sorted.filter(tag => tag.name.toLowerCase().includes(keyword))
  if (showAll.value) return sorted
  const top = sorted.slice(0, TOP_TAGS)
  const extra = filter.value.tags.map(tagById).filter((tag): tag is TagUsage => !!tag && !top.includes(tag))
  return [...extra, ...top]
})

const types: { value: MediaKind; label: string; icon: string }[] = [
  { value: 'all', label: '全部', icon: 'ri-apps-2-line' },
  { value: 'image', label: '图片', icon: 'ri-image-line' },
  { value: 'video', label: '视频', icon: 'ri-film-line' },
]
const roles: { value: UploaderRole; label: string; icon: string }[] = [
  { value: 'all', label: '全部', icon: 'ri-group-line' },
  { value: 'admin', label: '管理员上传', icon: 'ri-shield-user-line' },
  { value: 'user', label: '普通用户上传', icon: 'ri-user-line' },
]

const toggleTag = (id: number) => {
  filter.value.untagged = false
  filter.value.tags = filter.value.tags.includes(id) ? filter.value.tags.filter(item => item !== id) : [...filter.value.tags, id]
}
const toggleUntagged = () => {
  filter.value.tags = []
  filter.value.untagged = !filter.value.untagged
}
const clearTags = () => {
  filter.value.tags = []
  filter.value.untagged = false
}
const itemClass = (on: boolean) => [
  'relative flex h-8 w-full items-center gap-[9px] rounded-lg px-2.5 text-left text-[13.5px] transition-colors',
  'focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring [&>i]:text-base [&>i]:text-oi-muted',
  on ? 'bg-oi-raised font-[560] text-oi-ink shadow-[var(--oi-shadow-sm),0_0_0_1px_var(--oi-line)]' : 'text-oi-text hover:bg-oi-hover hover:text-oi-ink',
]
</script>

<template>
  <div class="font-oi text-sm">
    <h2 class="filter-title !pt-0">类型</h2>
    <div role="group" aria-label="类型" class="flex flex-col gap-0.5">
      <button
        v-for="item in types"
        :key="item.value"
        type="button"
        :aria-pressed="filter.type === item.value"
        :class="itemClass(filter.type === item.value)"
        @click="filter.type = item.value"
      >
        <i :class="item.icon" aria-hidden="true" />{{ item.label }}
      </button>
    </div>

    <h2 class="filter-title">
      标签
      <span class="flex gap-0.5">
        <button v-if="filter.tags.length || filter.untagged" type="button" class="filter-icon" aria-label="清除标签筛选" title="清除标签筛选" @click="clearTags"><i class="ri-filter-off-line" aria-hidden="true" /></button>
        <button v-if="canManageTags" type="button" class="filter-icon" aria-label="管理标签" title="管理标签" @click="emit('manage-tags')"><i class="ri-settings-4-line" aria-hidden="true" /></button>
      </span>
    </h2>
    <label v-if="tags.length > TOP_TAGS" class="mx-0.5 mb-1.5 flex h-8 items-center gap-1.5 rounded-lg border border-oi-line bg-oi-raised px-2 focus-within:border-oi-accent focus-within:shadow-[0_0_0_3px_var(--oi-ring)]">
      <i class="ri-search-line text-sm text-oi-faint" aria-hidden="true" />
      <input v-model="tagQuery" type="search" :placeholder="`在 ${tags.length} 个标签中查找`" aria-label="查找标签" class="min-w-0 flex-1 border-0 bg-transparent text-[12.5px] text-oi-ink outline-none placeholder:text-oi-faint">
    </label>
    <div :class="['flex flex-col gap-0.5', (showAll || tagQuery) && 'max-h-[300px] overflow-auto pr-1']">
      <button
        v-for="tag in railTags"
        :key="tag.id"
        type="button"
        role="checkbox"
        :aria-checked="filter.tags.includes(tag.id)"
        :class="itemClass(filter.tags.includes(tag.id))"
        @click="toggleTag(tag.id)"
      >
        <span :class="['grid h-4 w-4 flex-none place-items-center rounded-[5px] border-[1.5px] text-[11px]', filter.tags.includes(tag.id) ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']" aria-hidden="true"><i class="ri-check-line" /></span>
        <span class="h-[7px] w-[7px] flex-none rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />
        <span class="min-w-0 flex-1 truncate">{{ tag.name }}</span>
        <span class="text-xs tabular-nums text-oi-faint">{{ tag.count }}</span>
      </button>
      <p v-if="tagQuery && !railTags.length" class="px-2.5 pt-2 text-[11.5px] text-oi-faint">没有匹配的标签</p>
      <p v-if="!tags.length" class="px-2.5 pb-1 text-[11.5px] text-oi-faint">还没有标签</p>
    </div>
    <button
      v-if="!tagQuery && tags.length > TOP_TAGS"
      type="button"
      class="flex items-center gap-1 px-2.5 pb-2 pt-1 text-xs text-oi-muted hover:text-oi-accent-2"
      :aria-expanded="showAll"
      @click="showAll = !showAll"
    >
      {{ showAll ? '收起' : `显示全部 ${tags.length} 个` }}<i :class="showAll ? 'ri-arrow-up-s-line' : 'ri-arrow-down-s-line'" aria-hidden="true" />
    </button>
    <button type="button" role="checkbox" :aria-checked="filter.untagged" :class="itemClass(filter.untagged)" @click="toggleUntagged">
      <i class="ri-price-tag-line" aria-hidden="true" />未加标签
    </button>

    <template v-if="storages.length">
      <h2 class="filter-title">存储</h2>
      <div role="group" aria-label="存储" class="flex flex-col gap-0.5">
        <button
          v-for="storage in storages"
          :key="storage.id"
          type="button"
          :aria-pressed="filter.storage === storage.id"
          :class="itemClass(filter.storage === storage.id)"
          :title="storage.disabled ? '已停用：不接受新上传，已有文件照常访问' : undefined"
          @click="filter.storage = filter.storage === storage.id ? null : storage.id"
        >
          <StorageTypeIcon :type="storage.type" size="sm" class="!h-[18px] !w-[18px] !rounded-[5px] !text-[11px]" />
          <span class="min-w-0 flex-1 truncate">{{ storage.name }}</span>
          <span v-if="storage.disabled" class="rounded-full bg-oi-warn-soft px-1.5 text-[11px] text-oi-warn">已停用</span>
        </button>
      </div>
    </template>

    <template v-if="props.isAdmin">
      <h2 class="filter-title">上传者</h2>
      <div role="group" aria-label="上传者" class="flex flex-col gap-0.5">
        <button
          v-for="item in roles"
          :key="item.value"
          type="button"
          :aria-pressed="filter.role === item.value"
          :class="itemClass(filter.role === item.value)"
          @click="filter.role = item.value"
        >
          <i :class="item.icon" aria-hidden="true" />{{ item.label }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.filter-title {
  display: flex; align-items: center; justify-content: space-between; padding: 14px 10px 6px;
  font-size: 11.5px; font-weight: 550; letter-spacing: .02em; color: var(--oi-faint);
}
.filter-icon {
  display: grid; place-items: center; width: 22px; height: 22px; border-radius: 6px; font-size: 15px; color: var(--oi-faint);
}
.filter-icon:hover { background: var(--oi-hover); color: var(--oi-ink); }
.filter-icon:focus-visible { outline: none; box-shadow: 0 0 0 3px var(--oi-ring); }
</style>
