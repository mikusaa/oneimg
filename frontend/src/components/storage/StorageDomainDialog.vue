<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, ApiError, type StorageBucket } from '@/api/client.ts'
import { toast, UiButton, UiDialog, UiField, UiInput } from '@/components/ui'
import { domainError, normalizeDomain, sampleUrl } from './storageInfo'

const props = defineProps<{
  storage: StorageBucket | null
  /** 本地存储当前的 CDN 域名 */
  cdnDomain: string
}>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ saved: [] }>()

const value = ref('')
const serverError = ref('')
const saving = ref(false)

const local = computed(() => props.storage?.type === 'default')
const current = computed(() => {
  if (!props.storage) return ''
  return local.value ? props.cdnDomain : String(props.storage.config?.public_domain ?? '')
})
watch(open, isOpen => {
  if (!isOpen) return
  value.value = current.value
  serverError.value = ''
})

const title = computed(() => `${props.storage?.name ?? ''} · ${local.value ? 'CDN 域名' : '直链域名'}`)
const description = computed(() => (local.value
  ? 'CDN 的回源根路径要指向服务器的 uploads 目录，生成的链接会去掉 /uploads 前缀。'
  : '填写存储桶绑定的公开访问域名（如 R2 自定义域名）。浏览器会直接访问它，不经过 OneImg。'))

const filled = computed(() => value.value.trim() !== '')
const error = computed(() => domainError(value.value) || serverError.value)
const proxiedSample = computed(() => sampleUrl({ domain: '', stripUploads: false }))
const directSample = computed(() => sampleUrl({ domain: normalizeDomain(value.value) || 'https://img.example.com', stripUploads: local.value }))
const unchanged = computed(() => normalizeDomain(value.value) === normalizeDomain(current.value))

watch(value, () => { serverError.value = '' })

const save = async () => {
  const storage = props.storage
  if (!storage || error.value || saving.value) return
  if (unchanged.value) { open.value = false; return }
  const domain = normalizeDomain(value.value)
  saving.value = true
  try {
    if (local.value) {
      await api.updateSettings({ cdn_domain: domain })
    } else {
      // 只改直链域名不涉及端点和密钥，后端会保留其余配置
      await api.updateBucket(storage.id, {
        name: storage.name, type: storage.type, capacity_bytes: storage.capacity_bytes,
        config: { public_domain: domain },
      })
    }
    toast.success(domain ? '访问域名已更新' : '已改回经 OneImg 访问')
    open.value = false
    emit('saved')
  } catch (err) {
    if (err instanceof ApiError) {
      const field = err.problem?.errors?.find(item => item.field === 'config.public_domain')
      serverError.value = field?.message || err.problem?.detail || '保存失败'
    } else {
      serverError.value = '保存失败，请稍后重试'
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiDialog v-model:open="open" :title="title" :description="description" :icon="local ? 'ri-hard-drive-3-line' : 'ri-global-line'" wide>
    <form id="storage-domain-form" class="flex flex-col gap-3.5" @submit.prevent="save">
      <UiField :error="error" for="storage-domain-input">
        <UiInput
          id="storage-domain-input"
          v-model="value"
          mono
          :invalid="!!error"
          :aria-label="local ? 'CDN 域名' : '直链域名'"
          placeholder="https://img.example.com"
          autocomplete="off"
          spellcheck="false"
        />
      </UiField>
      <div class="grid grid-cols-2 gap-2 max-[640px]:grid-cols-1">
        <div :class="['flex min-w-0 flex-col gap-1.5 rounded-xl border px-3 py-2.5 transition-colors', !filled ? 'border-oi-accent bg-oi-accent-soft' : 'border-oi-line']">
          <span class="text-xs font-semibold text-oi-ink">留空 · 经 OneImg</span>
          <code class="break-all font-oi-mono text-[11px] text-oi-text">{{ proxiedSample }}</code>
          <span class="flex items-center gap-1 text-[11.5px] text-oi-muted"><i class="ri-shield-check-line" aria-hidden="true" />来源白名单生效，流量经过服务器</span>
        </div>
        <div :class="['flex min-w-0 flex-col gap-1.5 rounded-xl border px-3 py-2.5 transition-colors', filled ? 'border-oi-accent bg-oi-accent-soft' : 'border-oi-line']">
          <span class="text-xs font-semibold text-oi-ink">填写后 · 直连</span>
          <code class="break-all font-oi-mono text-[11px] text-oi-text">{{ directSample }}</code>
          <span class="flex items-center gap-1 text-[11.5px] text-oi-muted"><i class="ri-flashlight-line" aria-hidden="true" />速度更快，来源白名单不生效</span>
        </div>
      </div>
      <p class="flex gap-2 rounded-xl border border-oi-line bg-oi-warn-soft px-3 py-2.5 text-[12.5px] leading-normal text-oi-text">
        <i class="ri-error-warning-line text-[15px] leading-5 text-oi-warn" aria-hidden="true" />
        <span>链接在读取时生成：修改后，这个存储里所有文件（包括历史文件）显示的链接都会换成新域名。已经贴到外部的旧链接能否继续访问，取决于旧域名是否仍然可用。</span>
      </p>
    </form>
    <template #footer>
      <UiButton variant="ghost" @click="open = false">取消</UiButton>
      <UiButton type="submit" form="storage-domain-form" variant="primary" :disabled="!!domainError(value) || saving">
        <i v-if="saving" class="ri-loader-4-line animate-spin" aria-hidden="true" />保存
      </UiButton>
    </template>
  </UiDialog>
</template>
