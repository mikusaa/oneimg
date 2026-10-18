<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Image } from '@/api/client.ts'
import { toast, UiButton, UiDialog, UiSegmented } from '@/components/ui'
import { LINK_FORMATS, useLinkFormat, type LinkFormat } from '@/composables/useLinkFormat'
import { copyMediaText, mediaLink, safeMediaURL } from '@/utils/media'

const props = defineProps<{ images: Image[] }>()
const open = defineModel<boolean>('open', { default: false })

const linkFormat = useLinkFormat()
const format = ref<string>(linkFormat.value)
watch(open, value => { if (value) format.value = linkFormat.value })

const options = LINK_FORMATS.map(item => ({ value: item.value, label: item.label }))
const text = computed(() => props.images.map(image => mediaLink(image, format.value, safeMediaURL(image.url))).join('\n'))
const label = computed(() => LINK_FORMATS.find(item => item.value === format.value)?.label ?? 'URL')

const copy = async () => {
  try {
    await copyMediaText(text.value)
    linkFormat.value = format.value as LinkFormat
    toast.success(`已复制 ${props.images.length} 条 ${label.value} 链接`)
    open.value = false
  } catch {
    toast.error('复制失败，请手动复制')
  }
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    :title="`复制 ${images.length} 条链接`"
    description="每行一条。「动图式 HTML」会把视频输出为自动播放、循环、静音的 video 标签，图片按普通 HTML 输出。"
    icon="ri-file-copy-line"
    wide
  >
    <UiSegmented v-model="format" :options="options" aria-label="链接格式" class="flex-wrap self-start" />
    <textarea
      :value="text"
      readonly
      rows="8"
      aria-label="链接"
      class="w-full resize-y rounded-[10px] border border-oi-line-2 bg-oi-panel px-3 py-2 font-oi-mono text-[11.5px] text-oi-text outline-none focus:border-oi-accent focus:shadow-[0_0_0_3px_var(--oi-ring)]"
    />
    <template #footer="{ close }">
      <UiButton variant="ghost" @click="close">取消</UiButton>
      <UiButton variant="primary" @click="copy"><i class="ri-file-copy-line" aria-hidden="true" />复制</UiButton>
    </template>
  </UiDialog>
</template>
