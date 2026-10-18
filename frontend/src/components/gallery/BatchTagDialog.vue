<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type Image } from '@/api/client.ts'
import TagPicker from '@/components/tags/TagPicker.vue'
import { toast, UiButton, UiDialog, UiSegmented } from '@/components/ui'
import { loadTags } from '@/composables/useTags'

// 批量添加或移除标签；没有权限的文件跳过（后端接口为 add_tag_ids / remove_tag_ids，没有「替换」）

const props = defineProps<{
  images: Image[]
  can: (image: Image, permission: string) => boolean
  canCreateTag: boolean
}>()
const emit = defineEmits<{ done: [] }>()
const open = defineModel<boolean>('open', { default: false })

const mode = ref<'add' | 'remove'>('add')
const tagIds = ref<number[]>([])
const saving = ref(false)
watch(open, value => {
  if (!value) return
  mode.value = 'add'
  tagIds.value = []
})

const allowed = computed(() => props.images.filter(image => props.can(image, mode.value === 'add' ? 'image:tag:add' : 'image:tag:delete')))
const skipped = computed(() => props.images.length - allowed.value.length)
const modeOptions = [{ value: 'add', label: '添加标签' }, { value: 'remove', label: '移除标签' }]

const apply = async () => {
  if (!tagIds.value.length || saving.value) return
  if (!allowed.value.length) {
    toast.error('没有可以修改标签的文件')
    return
  }
  saving.value = true
  try {
    await api.updateImageTags({
      image_ids: allowed.value.map(image => image.id),
      ...(mode.value === 'add' ? { add_tag_ids: tagIds.value } : { remove_tag_ids: tagIds.value }),
    })
    toast.success(`已更新 ${allowed.value.length} 个文件${skipped.value ? `，${skipped.value} 个无权限已跳过` : ''}`)
    void loadTags(true).catch(() => undefined)
    open.value = false
    emit('done')
  } catch (error) {
    toast.error(error instanceof Error && error.message ? error.message : '修改标签失败')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    title="批量编辑标签"
    :description="`应用到已选的 ${images.length} 个文件；没有权限的文件会跳过。`"
    icon="ri-price-tag-3-line"
  >
    <UiSegmented v-model="mode" :options="modeOptions" aria-label="操作" class="self-start" />
    <div class="overflow-hidden rounded-xl border border-oi-line-2">
      <TagPicker v-model="tagIds" :allow-create="mode === 'add' && canCreateTag" />
    </div>
    <p v-if="skipped && tagIds.length" class="text-xs text-oi-warn">{{ skipped }} 个文件没有权限，会跳过</p>
    <template #footer="{ close }">
      <UiButton variant="ghost" @click="close">取消</UiButton>
      <UiButton :variant="mode === 'add' ? 'primary' : 'danger-solid'" :disabled="!tagIds.length || saving" @click="apply">
        {{ mode === 'add' ? '添加' : '移除' }}
      </UiButton>
    </template>
  </UiDialog>
</template>
