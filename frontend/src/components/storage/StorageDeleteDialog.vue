<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, ApiError, type StorageBucket } from '@/api/client.ts'
import { toast, UiButton, UiDialog, UiField, UiInput } from '@/components/ui'
import { formatCount } from '@/utils/format'

const props = defineProps<{
  storage: StorageBucket | null
  /** 可以改为停用（有 storage:update 且存储未停用） */
  canDisable: boolean
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ deleted: []; disable: [] }>()

const mode = ref<'keep' | 'purge'>('keep')
const typed = ref('')
const records = ref<number | null>(null)
const deleting = ref(false)
const error = ref('')
let countSeq = 0

watch(open, async isOpen => {
  if (!isOpen || !props.storage) return
  mode.value = 'keep'
  typed.value = ''
  error.value = ''
  records.value = null
  const seq = ++countSeq
  try {
    const page = await api.listImagesPage(`bucket_id=${props.storage.id}&page_size=1`)
    if (seq === countSeq) records.value = page.meta?.pagination?.total ?? null
  } catch { /* 读不到数量时只说明影响范围 */ }
})

const name = computed(() => props.storage?.name ?? '')
const matches = computed(() => typed.value.trim() === name.value)
const recordText = computed(() => (records.value == null ? '这个存储的全部图片与视频记录' : `${formatCount(records.value)} 条图片与视频记录`))

const CHOICES = [
  { value: 'keep', title: '保留远端文件', tag: '默认', description: '源文件和缩略图留在存储端，便于自行备份或迁移' },
  { value: 'purge', title: '同时删除远端文件', description: '清理这些记录对应、且未被其他记录引用的源文件和缩略图，无法恢复' },
] as const

const remove = async () => {
  if (!props.storage || !matches.value || deleting.value) return
  deleting.value = true
  error.value = ''
  try {
    await api.deleteBucket(props.storage.id, mode.value === 'purge')
    toast.success(mode.value === 'purge' ? '存储及远端文件已删除' : '存储已删除，远端文件已保留')
    open.value = false
    emit('deleted')
  } catch (err) {
    error.value = err instanceof ApiError ? err.problem?.detail || '删除失败' : '删除失败，请稍后重试'
  } finally {
    deleting.value = false
  }
}

const switchToDisable = () => {
  open.value = false
  emit('disable')
}
</script>

<template>
  <UiDialog v-model:open="open" :title="`删除存储「${name}」？`" icon="ri-delete-bin-6-line" tone="danger" wide>
    <template #header>
      <p class="mt-1 text-[13px] text-oi-muted">以下内容会被删除，无法撤销：</p>
      <ul class="mt-1.5 list-disc pl-[18px] text-[13px] leading-relaxed text-oi-text marker:text-oi-danger">
        <li>{{ recordText }}及其标签关联，这些文件的外链立即失效</li>
        <li>所有用户对它的「可用存储」授权</li>
      </ul>
    </template>

    <form id="storage-delete-form" class="flex flex-col gap-3.5" @submit.prevent="remove">
      <fieldset class="flex flex-col gap-2">
        <legend class="mb-1.5 text-[12.5px] font-[550] text-oi-text">远端文件</legend>
        <label
          v-for="choice in CHOICES"
          :key="choice.value"
          :class="[
            'flex cursor-pointer items-start gap-2.5 rounded-xl border px-3 py-2.5 transition-colors has-[:focus-visible]:ring-[3px] has-[:focus-visible]:ring-oi-ring',
            mode === choice.value ? (choice.value === 'purge' ? 'border-oi-danger bg-oi-danger-soft' : 'border-oi-accent bg-oi-accent-soft') : 'border-oi-line hover:border-oi-faint',
          ]"
        >
          <input v-model="mode" type="radio" name="storage-delete-mode" :value="choice.value" class="mt-[3px] accent-[var(--oi-accent)]">
          <span class="min-w-0">
            <span :class="['flex items-center gap-2 text-[13px] font-[600]', choice.value === 'purge' ? 'text-oi-danger' : 'text-oi-ink']">
              {{ choice.title }}
              <span v-if="'tag' in choice" class="rounded-full border border-oi-line px-1.5 text-[11px] font-[550] text-oi-muted">{{ choice.tag }}</span>
            </span>
            <span class="mt-0.5 block text-xs text-oi-muted">{{ choice.description }}</span>
          </span>
        </label>
      </fieldset>

      <p v-if="canDisable" class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12.5px] text-oi-muted">
        只想停止新上传、保留外链？
        <UiButton size="sm" variant="ghost" @click="switchToDisable"><i class="ri-pause-circle-line" aria-hidden="true" />改为停用</UiButton>
      </p>

      <UiField :label="`输入「${name}」确认`" for="storage-delete-confirm" :error="error">
        <UiInput id="storage-delete-confirm" v-model="typed" autocomplete="off" spellcheck="false" />
      </UiField>
    </form>

    <template #footer>
      <UiButton variant="ghost" @click="open = false">取消</UiButton>
      <UiButton type="submit" form="storage-delete-form" variant="danger-solid" :disabled="!matches || deleting">
        <i v-if="deleting" class="ri-loader-4-line animate-spin" aria-hidden="true" />{{ mode === 'purge' ? '删除存储和远端文件' : '删除存储' }}
      </UiButton>
    </template>
  </UiDialog>
</template>
