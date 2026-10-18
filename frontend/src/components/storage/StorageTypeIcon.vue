<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  /** 后端的存储类型：default（本地）、s3、r2、ftp、webdav */
  type: string
  size?: 'sm' | 'md'
}>(), { size: 'md' })

const TYPES: Record<string, { icon: string; background: string; label: string }> = {
  default: { icon: 'ri-hard-drive-3-line', background: 'linear-gradient(135deg,#4b5563,#8a94a3)', label: '本地' },
  s3: { icon: 'ri-archive-drawer-line', background: 'linear-gradient(135deg,#d6442f,#f0744f)', label: 'S3' },
  r2: { icon: 'ri-cloud-line', background: 'linear-gradient(135deg,#f6821f,#fbad41)', label: 'R2' },
  webdav: { icon: 'ri-folder-cloud-line', background: 'linear-gradient(135deg,#2f6fea,#5aa2ff)', label: 'WebDAV' },
  ftp: { icon: 'ri-server-line', background: 'linear-gradient(135deg,#7c4ddb,#a97ff7)', label: 'FTP' },
}

const meta = computed(() => TYPES[props.type.toLowerCase()] ?? { icon: 'ri-database-2-line', background: 'linear-gradient(135deg,#4b5563,#8a94a3)', label: props.type })
</script>

<template>
  <span
    :class="['grid flex-none place-items-center text-white', size === 'sm' ? 'h-[22px] w-[22px] rounded-md text-xs' : 'h-[38px] w-[38px] rounded-[11px] text-[19px]']"
    :style="{ background: meta.background }"
    :title="meta.label"
    aria-hidden="true"
  ><i :class="meta.icon" /></span>
</template>
