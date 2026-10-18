<script setup lang="ts">
import { computed, nextTick, onMounted, ref, useId, watch } from 'vue'
import type { TagUsage } from '@/api/client.ts'
import { toast } from '@/components/ui'
import { createTag, markTagUsed, tagById, tagColor, tagNameError, TAG_NAME_MAX, useTags } from '@/composables/useTags'

const props = withDefaults(defineProps<{
  /** 可以直接创建不存在的标签（需要 tag:create） */
  allowCreate?: boolean
  autofocus?: boolean
  /** 不显示已选标签（外部已经列出时） */
  hideSelected?: boolean
}>(), { allowCreate: false, autofocus: false, hideSelected: false })

const model = defineModel<number[]>({ default: () => [] })

const { tags, recentIds } = useTags()
const id = useId()
const input = ref<HTMLInputElement | null>(null)
const list = ref<HTMLElement | null>(null)
const query = ref('')
const active = ref(0)
const creating = ref(false)

const keyword = computed(() => query.value.trim().toLowerCase())
type Tag = TagUsage
const byName = (a: Tag, b: Tag) => a.name.localeCompare(b.name, 'zh')
// 使用次数多的排在前面，次数相同时按名称
const byUsage = (a: Tag, b: Tag) => b.count - a.count || byName(a, b)

const recent = computed(() => keyword.value ? [] : recentIds.value.map(tagById).filter((tag): tag is Tag => !!tag))
const matches = computed(() => {
  const k = keyword.value
  if (!k) return tags.value.filter(tag => !recentIds.value.includes(tag.id)).sort(byUsage)
  // 名称以关键词开头的排在前面
  const starts = (tag: Tag) => Number(tag.name.toLowerCase().startsWith(k))
  return tags.value.filter(tag => tag.name.toLowerCase().includes(k)).sort((a, b) => starts(b) - starts(a) || byUsage(a, b))
})
const options = computed(() => [...recent.value, ...matches.value])
const canCreate = computed(() => props.allowCreate && !!keyword.value && !tags.value.some(tag => tag.name.toLowerCase() === keyword.value))
const total = computed(() => options.value.length + (canCreate.value ? 1 : 0))
const selected = computed(() => model.value.map(tagById).filter((tag): tag is Tag => !!tag))

const optionId = (index: number) => `${id}-option-${index}`
const activeId = computed(() => total.value ? optionId(active.value) : undefined)

// 关键词中匹配的部分高亮显示；按文本渲染，不使用 v-html
const parts = (name: string) => {
  const k = keyword.value
  const index = k ? name.toLowerCase().indexOf(k) : -1
  if (index < 0) return [{ text: name, hit: false }]
  return [
    { text: name.slice(0, index), hit: false },
    { text: name.slice(index, index + k.length), hit: true },
    { text: name.slice(index + k.length), hit: false },
  ].filter(part => part.text)
}

const toggle = (tag: Tag) => {
  if (model.value.includes(tag.id)) {
    model.value = model.value.filter(item => item !== tag.id)
  } else {
    model.value = [...model.value, tag.id]
    markTagUsed(tag.id)
  }
}

const create = async () => {
  const name = query.value.trim()
  if (!name || creating.value) return
  const error = tagNameError(name)
  if (error) {
    toast.error(error)
    return
  }
  creating.value = true
  try {
    const tag = await createTag(name)
    model.value = [...model.value, tag.id]
    markTagUsed(tag.id)
    query.value = ''
    toast.success(`已创建标签「${tag.name}」`)
  } catch (error) {
    toast.error(error instanceof Error && error.message ? error.message : '创建标签失败')
  } finally {
    creating.value = false
  }
}

const scrollToActive = () => nextTick(() => {
  list.value?.parentElement?.querySelector(`#${CSS.escape(optionId(active.value))}`)?.scrollIntoView({ block: 'nearest' })
})

const onKeydown = (event: KeyboardEvent) => {
  if (event.isComposing || !total.value) return
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    active.value = (active.value + 1) % total.value
    scrollToActive()
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    active.value = (active.value - 1 + total.value) % total.value
    scrollToActive()
  } else if (event.key === 'Enter') {
    event.preventDefault()
    if (active.value < options.value.length) toggle(options.value[active.value])
    else void create()
  }
}

watch(keyword, () => { active.value = 0 })
watch(total, value => { if (active.value >= value) active.value = 0 })

onMounted(() => {
  if (props.autofocus) nextTick(() => input.value?.focus())
})
</script>

<template>
  <div class="flex flex-col font-oi">
    <div class="flex h-[42px] items-center gap-2 border-b border-oi-line px-3">
      <i class="ri-search-line text-base text-oi-faint" aria-hidden="true" />
      <input
        ref="input"
        v-model="query"
        type="text"
        role="combobox"
        aria-expanded="true"
        :aria-controls="`${id}-list`"
        :aria-activedescendant="activeId"
        aria-autocomplete="list"
        aria-label="搜索标签"
        :placeholder="`搜索 ${tags.length} 个标签${allowCreate ? '，或输入新名称' : ''}`"
        :maxlength="TAG_NAME_MAX * 2"
        class="min-w-0 flex-1 border-0 bg-transparent text-[13.5px] text-oi-ink outline-none placeholder:text-oi-faint"
        @keydown="onKeydown"
      >
    </div>

    <div v-if="selected.length && !hideSelected" class="flex max-h-[92px] flex-wrap gap-1 overflow-auto border-b border-oi-line px-2.5 py-2">
      <span
        v-for="tag in selected"
        :key="tag.id"
        class="inline-flex h-6 max-w-full items-center gap-[5px] whitespace-nowrap rounded-full border border-oi-line bg-oi-panel pl-2 pr-1 text-xs text-oi-ink"
      >
        <span class="h-[7px] w-[7px] flex-none rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />
        <span class="truncate">{{ tag.name }}</span>
        <button
          type="button"
          class="grid h-[18px] w-[18px] flex-none place-items-center rounded-full text-[13px] text-oi-muted hover:bg-oi-line hover:text-oi-ink"
          :aria-label="`移除 ${tag.name}`"
          @click="toggle(tag)"
        ><i class="ri-close-line" aria-hidden="true" /></button>
      </span>
    </div>

    <div class="max-h-[260px] overflow-auto px-1.5 pb-1.5 pt-1">
      <div :id="`${id}-list`" ref="list" role="listbox" aria-multiselectable="true" aria-label="标签">
        <div v-if="recent.length" role="group" :aria-labelledby="`${id}-recent`">
          <div :id="`${id}-recent`" class="px-2 pb-1 pt-2 text-[11px] text-oi-faint">最近使用</div>
          <div
            v-for="(tag, index) in recent"
            :id="optionId(index)"
            :key="`recent-${tag.id}`"
            role="option"
            :aria-selected="model.includes(tag.id)"
            :class="['flex h-8 cursor-pointer items-center gap-[9px] rounded-lg px-2 text-[13px]', active === index ? 'bg-oi-hover text-oi-ink' : 'text-oi-text']"
            @mouseenter="active = index"
            @mousedown.prevent
            @click="toggle(tag)"
          >
            <span :class="['grid h-4 w-4 flex-none place-items-center rounded-[5px] border-[1.5px] text-[11px] transition-colors', model.includes(tag.id) ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']" aria-hidden="true"><i class="ri-check-line" /></span>
            <span class="h-[7px] w-[7px] flex-none rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />
            <span class="min-w-0 flex-1 truncate">{{ tag.name }}</span>
            <span class="text-[11.5px] tabular-nums text-oi-faint">{{ tag.count }}</span>
          </div>
        </div>

        <div role="group" :aria-labelledby="`${id}-all`">
          <div :id="`${id}-all`" class="px-2 pb-1 pt-2 text-[11px] text-oi-faint">{{ keyword ? `匹配 ${matches.length} 个` : '全部 · 按使用次数' }}</div>
          <div
            v-for="(tag, index) in matches"
            :id="optionId(index + recent.length)"
            :key="tag.id"
            role="option"
            :aria-selected="model.includes(tag.id)"
            :class="['flex h-8 cursor-pointer items-center gap-[9px] rounded-lg px-2 text-[13px]', active === index + recent.length ? 'bg-oi-hover text-oi-ink' : 'text-oi-text']"
            @mouseenter="active = index + recent.length"
            @mousedown.prevent
            @click="toggle(tag)"
          >
            <span :class="['grid h-4 w-4 flex-none place-items-center rounded-[5px] border-[1.5px] text-[11px] transition-colors', model.includes(tag.id) ? 'border-oi-accent bg-oi-accent text-white' : 'border-oi-line-2 bg-oi-raised text-transparent']" aria-hidden="true"><i class="ri-check-line" /></span>
            <span class="h-[7px] w-[7px] flex-none rounded-full" :style="{ background: tagColor(tag.name) }" aria-hidden="true" />
            <span class="min-w-0 flex-1 truncate">
              <template v-for="(part, partIndex) in parts(tag.name)" :key="partIndex">
                <mark v-if="part.hit" class="rounded-[3px] bg-oi-accent-soft px-px text-oi-accent-2">{{ part.text }}</mark>
                <template v-else>{{ part.text }}</template>
              </template>
            </span>
            <span class="text-[11.5px] tabular-nums text-oi-faint">{{ tag.count }}</span>
          </div>
        </div>
      </div>
      <div v-if="!matches.length && !canCreate" class="p-[18px] text-center text-[12.5px] text-oi-muted">
        {{ tags.length ? '没有匹配的标签' : '还没有标签' }}
      </div>
    </div>

    <button
      v-if="canCreate"
      :id="optionId(options.length)"
      type="button"
      tabindex="-1"
      :disabled="creating"
      :class="['flex w-full items-center gap-2 border-t border-oi-line px-3.5 py-2.5 text-left text-[13px] font-[550] text-oi-accent-2 hover:bg-oi-accent-soft disabled:opacity-60', active === options.length ? 'bg-oi-accent-soft' : '']"
      @mouseenter="active = options.length"
      @mousedown.prevent
      @click="create"
    >
      <i :class="creating ? 'ri-loader-4-line animate-spin' : 'ri-add-line'" aria-hidden="true" />
      <span class="min-w-0 flex-1 truncate">创建标签「{{ query.trim() }}」</span>
      <kbd class="inline-flex h-[18px] items-center rounded-[5px] border border-oi-line bg-oi-panel px-[5px] font-oi-mono text-[11px] text-oi-muted">↵</kbd>
    </button>
  </div>
</template>
