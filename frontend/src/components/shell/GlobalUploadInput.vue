<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { toast } from '@/components/ui'
import { useSession } from '@/composables/useSession'
import { addFiles } from '@/composables/useUploadQueue'
import { NAV_ITEMS } from '@/router/nav'

// 外壳上的全局输入：任意页面拖入或粘贴文件都加入上传队列，U / G 快捷键切换到上传和图库
const route = useRoute()
const router = useRouter()
const { mustChangePassword } = useSession()

const overlay = ref(false)
let depth = 0

const hasFiles = (event: DragEvent) => [...(event.dataTransfer?.types || [])].includes('Files')
const isEditable = (target: EventTarget | null) =>
  target instanceof HTMLElement && (target.isContentEditable || !!target.closest('input, textarea, select, [contenteditable]'))
// 弹窗、菜单或气泡打开时不响应，避免误操作
const overlayOpen = () => [...document.querySelectorAll('[role="dialog"], [role="alertdialog"], [role="menu"]')]
  .some(element => element.getClientRects().length > 0)

const toUpload = async () => {
  if (route.name !== 'Upload') await router.push('/upload')
}

const receive = async (files: File[], source: string) => {
  await toUpload()
  const added = await addFiles(files)
  if (added) toast.success(`已${source}添加 ${added} 个文件`)
}

const onPaste = (event: ClipboardEvent) => {
  if (mustChangePassword.value || isEditable(event.target) || overlayOpen()) return
  const files = [...(event.clipboardData?.files || [])]
  if (!files.length) return
  event.preventDefault()
  void receive(files, '从剪贴板')
}

const onDragEnter = (event: DragEvent) => {
  if (!hasFiles(event)) return
  depth += 1
  if (!mustChangePassword.value && route.name !== 'Upload' && !overlayOpen()) overlay.value = true
}
const onDragLeave = (event: DragEvent) => {
  if (!hasFiles(event)) return
  depth = Math.max(0, depth - 1)
  if (!depth) overlay.value = false
}
// 始终拦截文件拖放，避免浏览器直接打开文件、离开应用
const onDragOver = (event: DragEvent) => {
  if (!hasFiles(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = mustChangePassword.value || overlayOpen() ? 'none' : 'copy'
}
const onDrop = (event: DragEvent) => {
  if (!hasFiles(event)) return
  event.preventDefault()
  depth = 0
  overlay.value = false
  if (mustChangePassword.value || overlayOpen()) return
  const files = [...(event.dataTransfer?.files || [])]
  if (files.length) void receive(files, '')
}

const SHORTCUTS = Object.fromEntries(NAV_ITEMS.filter(item => item.shortcut).map(item => [item.shortcut!.toLowerCase(), item.path]))

const onKeydown = (event: KeyboardEvent) => {
  if (event.defaultPrevented || event.repeat || event.isComposing || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return
  if (mustChangePassword.value || isEditable(event.target) || overlayOpen()) return
  const path = SHORTCUTS[event.key.toLowerCase()]
  if (!path || route.path === path) return
  event.preventDefault()
  void router.push(path)
}

onMounted(() => {
  window.addEventListener('paste', onPaste)
  window.addEventListener('dragenter', onDragEnter)
  window.addEventListener('dragleave', onDragLeave)
  window.addEventListener('dragover', onDragOver)
  window.addEventListener('drop', onDrop)
  window.addEventListener('keydown', onKeydown)
})
onBeforeUnmount(() => {
  window.removeEventListener('paste', onPaste)
  window.removeEventListener('dragenter', onDragEnter)
  window.removeEventListener('dragleave', onDragLeave)
  window.removeEventListener('dragover', onDragOver)
  window.removeEventListener('drop', onDrop)
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Transition enter-from-class="opacity-0" leave-to-class="opacity-0" enter-active-class="transition-opacity duration-150" leave-active-class="transition-opacity duration-150">
    <div
      v-if="overlay"
      class="pointer-events-none fixed inset-0 z-[150] grid place-items-center bg-[color-mix(in_srgb,var(--oi-accent)_14%,var(--oi-scrim))] font-oi backdrop-blur-[4px]"
      aria-hidden="true"
    >
      <div class="rounded-3xl border-2 border-dashed border-oi-accent bg-oi-raised px-10 py-7 text-center shadow-oi-lg">
        <i class="ri-upload-cloud-2-line text-[34px] text-oi-accent" />
        <b class="mt-2 block text-[17px] text-oi-ink">松开即可添加</b>
        <span class="text-oi-muted">加入待上传列表</span>
      </div>
    </div>
  </Transition>
</template>
