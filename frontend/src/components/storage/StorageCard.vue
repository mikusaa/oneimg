<script setup lang="ts">
import { computed } from 'vue'
import type { StorageBucket } from '@/api/client.ts'
import { UiButton } from '@/components/ui'
import { formatBytes } from '@/utils/format'
import StorageTypeIcon from './StorageTypeIcon.vue'
import { sampleUrl, storageAccess, storageLocation, supportsDirectDomain, TYPE_LABELS } from './storageInfo'

const props = defineProps<{
  storage: StorageBucket
  isDefault: boolean
  /** 本地存储的 CDN 域名（系统设置 cdn_domain）；没有 setting:upload 权限时读不到，为 null */
  cdnDomain: string | null
  canUpdate: boolean
  canDelete: boolean
  /** setting:upload：修改本地 CDN 域名 */
  canEditCdn: boolean
  busy?: boolean
}>()

const emit = defineEmits<{
  edit: []
  domain: []
  test: []
  toggle: []
  delete: []
}>()

const local = computed(() => props.storage.type === 'default')
const access = computed(() => storageAccess(props.storage, props.cdnDomain ?? ''))
// 读不到本地 CDN 域名时不显示访问方式，避免误报「经 OneImg」
const accessUnknown = computed(() => local.value && props.cdnDomain === null)
const link = computed(() => sampleUrl(access.value))
const capacity = computed(() => Number(props.storage.capacity_bytes) || 0)
const usage = computed(() => Number(props.storage.usage_bytes) || 0)
const full = computed(() => !local.value && capacity.value > 0 && usage.value >= capacity.value)

const usageCells = computed(() => {
  if (local.value) {
    const fs = props.storage.filesystem
    return [
      { label: '磁盘容量', value: formatBytes(fs?.total_bytes) },
      { label: 'OneImg 占用', value: formatBytes(usage.value), estimated: !props.storage.usage_exact },
      { label: '目录可用', value: formatBytes(fs?.available_bytes) },
    ]
  }
  return [
    { label: '配置容量', value: capacity.value > 0 ? formatBytes(capacity.value) : '无限制' },
    { label: 'OneImg 占用', value: formatBytes(usage.value), estimated: !props.storage.usage_exact },
    { label: '剩余配额', value: capacity.value > 0 ? formatBytes(Math.max(0, capacity.value - usage.value)) : '无限制' },
  ]
})

// 本地存储显示磁盘使用率（含其他程序占用），远端存储显示配额使用率
const percent = computed<number | null>(() => {
  if (local.value) {
    const fs = props.storage.filesystem
    return fs?.total_bytes ? Math.min(100, fs.used_bytes / fs.total_bytes * 100) : null
  }
  return capacity.value > 0 ? Math.min(100, usage.value / capacity.value * 100) : null
})
const percentText = computed(() => {
  if (percent.value == null) return local.value ? '无法读取磁盘容量' : '未设置容量上限'
  const value = percent.value > 0 && percent.value < 1 ? '< 1' : Math.round(percent.value)
  return local.value ? `磁盘使用率 ${value}%（含其他程序占用）` : `配额使用率 ${value}%`
})

const directLabel = computed(() => (local.value ? 'CDN 直连' : '直链直连'))
const accessHint = computed(() => {
  if (local.value) return access.value.direct ? 'CDN 回源到 uploads 目录，链接去掉 /uploads 前缀' : '可以设置 CDN 域名加速访问'
  if (supportsDirectDomain(props.storage.type)) return access.value.direct ? '浏览器直接访问存储桶的公开域名' : '可以设置存储桶的公开域名，不经 OneImg 直连'
  return `${TYPE_LABELS[props.storage.type] ?? props.storage.type} 没有公开地址，始终经 OneImg 读取`
})
const canEditDomain = computed(() => (local.value ? props.canEditCdn : supportsDirectDomain(props.storage.type) && props.canUpdate))
const domainButton = computed(() => {
  if (access.value.direct) return '修改'
  return local.value ? '设置 CDN 域名' : '设置直链域名'
})
</script>

<template>
  <article
    :class="[
      'flex flex-col gap-3.5 rounded-oi-lg border bg-oi-raised p-[18px] transition-shadow hover:shadow-oi',
      isDefault ? 'border-[color-mix(in_srgb,var(--oi-accent)_45%,var(--oi-line))]' : 'border-oi-line',
    ]"
    :aria-labelledby="`storage-${storage.id}-name`"
  >
    <div class="flex items-start gap-3">
      <StorageTypeIcon :type="storage.type" :class="storage.disabled && 'opacity-55 grayscale'" />
      <div class="min-w-0 flex-1">
        <h2 :id="`storage-${storage.id}-name`" class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-[620]" :class="storage.disabled ? 'text-oi-text' : 'text-oi-ink'">
          <span class="min-w-0 break-all">{{ storage.name }}</span>
          <span v-if="isDefault" class="rounded-full bg-oi-accent-soft px-2 text-[11.5px] font-[550] leading-5 text-oi-accent-2">默认</span>
          <span v-if="local" class="rounded-full border border-oi-line px-2 text-[11.5px] font-[550] leading-[18px] text-oi-muted">内置</span>
          <span v-if="storage.disabled" class="inline-flex items-center gap-1 rounded-full bg-oi-warn-soft px-2 text-[11.5px] font-[550] leading-5 text-oi-warn">
            <i class="ri-pause-circle-line" aria-hidden="true" />已停用
          </span>
          <span v-else-if="full" class="rounded-full bg-oi-warn-soft px-2 text-[11.5px] font-[550] leading-5 text-oi-warn">已满 · 暂停上传</span>
        </h2>
        <p class="mt-[3px] truncate font-oi-mono text-xs text-oi-muted" :title="storageLocation(storage)">{{ storageLocation(storage) }}</p>
      </div>
    </div>

    <p v-if="storage.disabled" class="flex items-start gap-2 rounded-[10px] border border-oi-line bg-oi-panel px-[11px] py-[9px] text-[12.5px] leading-normal text-oi-text">
      <i class="ri-information-line text-[15px] leading-5 text-oi-muted" aria-hidden="true" />
      <span>不接受新上传，也不出现在上传选项中；已有文件和外链照常可用。</span>
    </p>

    <dl class="grid grid-cols-3 gap-2">
      <div v-for="cell in usageCells" :key="cell.label" class="flex min-w-0 flex-col gap-0.5">
        <dt class="text-[11.5px] text-oi-muted">{{ cell.label }}</dt>
        <dd class="flex items-center gap-1 whitespace-nowrap text-sm font-[620] tabular-nums text-oi-ink">
          {{ cell.value }}
          <em
            v-if="cell.estimated"
            class="rounded px-[5px] py-px text-[10px] font-semibold not-italic text-oi-warn bg-oi-warn-soft"
            title="部分历史文件缺少计量，按估算值显示"
          >估算</em>
        </dd>
      </div>
    </dl>
    <div>
      <div
        class="flex h-1.5 overflow-hidden rounded-full bg-oi-line"
        role="meter"
        :aria-valuenow="percent == null ? undefined : Math.round(percent)"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-label="local ? '磁盘使用率' : '配额使用率'"
      >
        <span
          class="h-full transition-[width] duration-500"
          :class="[full ? 'bg-oi-warn' : 'bg-oi-accent', storage.disabled && 'opacity-55 grayscale']"
          :style="{ width: `${percent ?? 0}%` }"
        />
      </div>
      <p class="mt-1.5 text-[11.5px] text-oi-muted">{{ percentText }}</p>
    </div>

    <section class="flex flex-col gap-1.5 rounded-xl border border-oi-line bg-oi-panel px-3 py-2.5" :aria-label="`${storage.name}的访问地址`">
      <div class="flex items-center justify-between gap-2 text-xs font-semibold text-oi-text">
        <span>访问地址</span>
        <span v-if="accessUnknown" class="text-[11.5px] font-normal text-oi-muted">未知</span>
        <span
          v-else-if="access.direct"
          class="inline-flex items-center gap-1 rounded-full bg-oi-sakura-soft px-2 text-[11.5px] font-[550] leading-5 text-oi-sakura"
          title="浏览器直接访问该域名，不经过 OneImg，来源白名单不生效"
        ><i class="ri-flashlight-line" aria-hidden="true" />{{ directLabel }}</span>
        <span
          v-else
          class="inline-flex items-center gap-1 rounded-full bg-oi-accent-soft px-2 text-[11.5px] font-[550] leading-5 text-oi-accent-2"
          title="经 OneImg 读取，来源白名单生效"
        ><i class="ri-shield-check-line" aria-hidden="true" />经 OneImg</span>
      </div>
      <p v-if="accessUnknown" class="text-[11.5px] text-oi-muted">需要「存储与上传」设置权限才能查看 CDN 域名。</p>
      <p v-else class="truncate font-oi-mono text-[11.5px] text-oi-ink" :title="`链接示例：${link}`">{{ link }}</p>
      <div v-if="!accessUnknown" class="flex min-h-[30px] items-center justify-between gap-2 text-[11.5px] text-oi-muted">
        <span class="min-w-0">{{ accessHint }}</span>
        <UiButton v-if="canEditDomain" variant="ghost" size="sm" :disabled="busy" @click="emit('domain')">{{ domainButton }}</UiButton>
      </div>
    </section>

    <div v-if="!local && (canUpdate || canDelete)" class="mt-auto flex flex-wrap gap-1.5 border-t border-oi-line pt-3">
      <UiButton v-if="canUpdate" size="sm" :disabled="busy" @click="emit('edit')"><i class="ri-edit-line" aria-hidden="true" />编辑</UiButton>
      <UiButton v-if="canUpdate" variant="ghost" size="sm" :disabled="busy" @click="emit('test')"><i class="ri-pulse-line" aria-hidden="true" />测试连接</UiButton>
      <span class="flex-1" />
      <UiButton
        v-if="canUpdate"
        variant="ghost"
        size="sm"
        :disabled="busy"
        :title="storage.disabled ? '重新接受新上传' : '停止新上传，保留已有文件和外链'"
        @click="emit('toggle')"
      >
        <i :class="storage.disabled ? 'ri-play-circle-line' : 'ri-pause-circle-line'" aria-hidden="true" />{{ storage.disabled ? '启用' : '停用' }}
      </UiButton>
      <UiButton v-if="canDelete" variant="danger" size="sm" icon :disabled="busy" aria-label="删除存储" title="删除存储" @click="emit('delete')">
        <i class="ri-delete-bin-6-line" aria-hidden="true" />
      </UiButton>
    </div>
    <p v-else-if="local" class="mt-auto flex items-start gap-1.5 border-t border-oi-line pt-3 text-xs leading-[18px] text-oi-muted">
      <i class="ri-information-line" aria-hidden="true" />
      <span>内置存储，文件写在 <span class="font-oi-mono">uploads</span> 目录，不能停用或删除</span>
    </p>
  </article>
</template>
