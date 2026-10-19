<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { UiButton, UiInput, UiMenu, UiMenuItem } from '@/components/ui'
import { fixTemplate, joinTokens, TEMPLATE_MENU, TEMPLATE_VARS, tokenize, validateTemplate, type TemplateIssue, type TemplateKind, type TemplateToken } from './template'

// 存储路径 / 文件名拼装器：从菜单选变量或加固定文字，也可以切到直接编辑，两边同步
const props = defineProps<{
  kind: TemplateKind
  /** 输入框和拼装区的 id 前缀 */
  id: string
}>()
const model = defineModel<string>({ required: true })

const isPath = computed(() => props.kind === 'path')
const label = computed(() => isPath.value ? '存储路径' : '文件名模板')
const manual = ref(false)
const editing = ref<number | 'new' | -1>(-1)
const editText = ref('')
const note = ref('')

const tokens = computed(() => {
  const list = tokenize(model.value.trim())
  if (isPath.value) while (list[0]?.t === 'sep') list.shift()
  return list
})
// 路径开头的 uploads 显示为锁定：本地存储始终放在 uploads 下，保留这一级让各存储目录一致
const locked = (index: number) => {
  const token = tokens.value[0]
  return isPath.value && index === 0 && token?.t === 'text' && token.v === 'uploads'
}

const emitTokens = (list: TemplateToken[]) => {
  let value = joinTokens(list)
  if (isPath.value) value = `/${value.replace(/\/{2,}/g, '/').replace(/^\/+|\/+$/g, '')}`
  model.value = value
}
const removeAt = (index: number) => {
  const list = [...tokens.value]
  // 同时去掉相邻的 /
  if (isPath.value) {
    const neighbor = list[index - 1]?.t === 'sep' ? index - 1 : list[index + 1]?.t === 'sep' ? index + 1 : -1
    list.splice(index, 1)
    if (neighbor >= 0) list.splice(neighbor > index ? index : neighbor, 1)
  } else list.splice(index, 1)
  emitTokens(list)
}

const sanitize = (value: string) => {
  const trimmed = value.trim()
  const out = trimmed.replace(/\s+/g, '-').replace(isPath.value ? /[^A-Za-z0-9_-]/g : /[^A-Za-z0-9_.-]/g, '')
  note.value = out !== trimmed ? `固定文字只能用字母、数字、${isPath.value ? '' : '.、'}- 和 _，已自动处理` : ''
  return out
}
// 编辑框在 v-for 里，按 id 查找
const focusEditor = () => nextTick(() => document.getElementById(`${props.id}-edit`)?.focus())
const startEdit = (index: number) => {
  const token = tokens.value[index]
  if (token?.t !== 'text') return
  editing.value = index
  editText.value = token.v
  focusEditor()
}
const commitEdit = () => {
  const index = editing.value
  if (index === -1) return
  editing.value = -1
  const value = sanitize(editText.value)
  const list = [...tokens.value]
  if (index === 'new') {
    if (!value) return
    if (isPath.value && list.length) list.push({ t: 'sep' })
    list.push({ t: 'text', v: value })
  } else if (value) list[index] = { t: 'text', v: value }
  else list.splice(index, 1)
  emitTokens(list)
}
const onEditKey = (event: KeyboardEvent) => {
  if (event.isComposing) return
  if (event.key === 'Enter') { event.preventDefault(); commitEdit() }
  if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); editing.value = -1 }
}
const addVariable = (key: string | null) => {
  if (!key) {
    editing.value = 'new'
    editText.value = ''
    focusEditor()
    return
  }
  const list = [...tokens.value]
  if (isPath.value && list.length) list.push({ t: 'sep' })
  list.push({ t: 'var', k: key })
  note.value = ''
  emitTokens(list)
}

const issues = computed(() => validateTemplate(model.value, props.kind))
const fixableCount = computed(() => issues.value.filter(issue => issue.fix && issue.level !== 'info').length)
const applyFix = (issue: TemplateIssue) => { if (issue.fix) model.value = issue.fix(model.value) }
const fixAll = () => { model.value = fixTemplate(model.value, props.kind) }
const variableKnown = (key: string) => !!TEMPLATE_VARS[key]
// 插值里不能直接写 `{${key}}`，会和 }} 冲突
const braced = (key: string) => `{${key}}`
const textValid = (value: string) => (isPath.value ? /^[A-Za-z0-9_.-]+$/ : /^[^\\/:"'<>|?*\s]+$/).test(value) && !(isPath.value && (value === '.' || value === '..'))
const menu = computed(() => TEMPLATE_MENU[props.kind])
const issueIcon = (level: TemplateIssue['level']) => level === 'error' ? 'ri-error-warning-fill text-oi-danger' : level === 'warn' ? 'ri-alert-line text-oi-warn' : 'ri-information-line text-oi-muted'
const chip = 'inline-flex h-[30px] items-center gap-[5px] rounded-lg border bg-oi-raised text-[13px] text-oi-ink shadow-oi-sm'
const chipRemove = 'grid h-6 w-6 place-items-center rounded-md text-oi-muted hover:bg-oi-hover hover:text-oi-ink focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring'
</script>

<template>
  <div class="flex flex-col gap-2.5">
    <div
      v-if="!manual"
      :id="`${id}-chips`"
      role="group"
      :aria-label="label"
      class="flex min-h-[46px] flex-wrap items-center gap-1.5 rounded-xl border border-oi-line-2 bg-oi-panel px-2.5 py-2"
    >
      <span v-if="isPath" class="font-oi-mono text-[15px] text-oi-faint" aria-hidden="true">/</span>
      <template v-for="(token, index) in tokens" :key="index">
        <span v-if="token.t === 'sep'" class="font-oi-mono text-[15px] text-oi-faint" aria-hidden="true">/</span>
        <span
          v-else-if="locked(index)"
          class="inline-flex h-[30px] items-center gap-[5px] rounded-lg border border-dashed border-oi-line-2 px-[9px] font-oi-mono text-[12.5px] text-oi-muted"
          title="本地存储的文件始终放在 uploads 目录下；保留这一级可以让各存储的目录结构一致"
        ><i class="ri-lock-line text-[13px]" aria-hidden="true" />uploads</span>
        <span
          v-else-if="token.t === 'var'"
          :class="[chip, 'pl-[9px] pr-[3px]', variableKnown(token.k) ? 'border-[color-mix(in_srgb,var(--oi-accent)_40%,var(--oi-line-2))]' : 'border-oi-danger bg-oi-danger-soft']"
        >
          <b :class="['font-[560]', !variableKnown(token.k) && 'text-oi-danger']">{{ variableKnown(token.k) ? TEMPLATE_VARS[token.k].label : '未知' }}</b>
          <code :class="['font-oi-mono text-[11px]', variableKnown(token.k) ? 'text-oi-muted' : 'text-oi-danger']">{{ braced(token.k) }}</code>
          <button type="button" :class="chipRemove" :aria-label="`移除 {${token.k}}`" @click="removeAt(index)"><i class="ri-close-line" aria-hidden="true" /></button>
        </span>
        <input
          v-else-if="editing === index"
          :id="`${id}-edit`"
          v-model="editText"
          class="h-7 rounded-lg border border-oi-accent bg-oi-raised px-2 font-oi-mono text-[12.5px] text-oi-ink shadow-[0_0_0_3px_var(--oi-ring)] outline-none"
          :size="Math.max(6, editText.length + 1)"
          :placeholder="isPath ? '目录名' : '文字'"
          :aria-label="isPath ? '目录名' : '固定文字'"
          @blur="commitEdit"
          @keydown="onEditKey"
        >
        <span v-else :class="[chip, 'pl-1 pr-[3px]', textValid(token.v) ? 'border-oi-line-2' : 'border-oi-danger bg-oi-danger-soft']">
          <button type="button" class="rounded px-0.5 font-oi-mono text-[12.5px] text-oi-ink focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring" :title="`修改「${token.v}」`" @click="startEdit(index)">{{ token.v }}</button>
          <button type="button" :class="chipRemove" :aria-label="`移除 ${token.v}`" @click="removeAt(index)"><i class="ri-close-line" aria-hidden="true" /></button>
        </span>
      </template>
      <template v-if="editing === 'new'">
        <span v-if="isPath && tokens.length" class="font-oi-mono text-[15px] text-oi-faint" aria-hidden="true">/</span>
        <input
          :id="`${id}-edit`"
          v-model="editText"
          class="h-7 rounded-lg border border-oi-accent bg-oi-raised px-2 font-oi-mono text-[12.5px] text-oi-ink shadow-[0_0_0_3px_var(--oi-ring)] outline-none"
          :size="Math.max(6, editText.length + 1)"
          :placeholder="isPath ? '目录名' : '文字'"
          :aria-label="isPath ? '新目录名' : '新固定文字'"
          @blur="commitEdit"
          @keydown="onEditKey"
        >
      </template>
      <UiMenu v-else>
        <template #trigger>
          <button
            type="button"
            class="inline-flex h-7 items-center gap-1 rounded-lg border border-dashed border-oi-line-2 px-2.5 text-[12.5px] font-[550] text-oi-accent-2 hover:border-oi-accent hover:bg-oi-accent-soft focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
          >
            <i class="ri-add-line" aria-hidden="true" />{{ isPath ? (tokens.length ? '加一级目录' : '添加目录') : '添加' }}
          </button>
        </template>
        <UiMenuItem v-for="key in menu" :key="key" @select="addVariable(key)">
          {{ TEMPLATE_VARS[key].label }}
          <template #end><code class="font-oi-mono text-[11.5px] text-oi-muted">{{ TEMPLATE_VARS[key].sample }}</code></template>
        </UiMenuItem>
        <UiMenuItem icon="ri-text" @select="addVariable(null)">
          固定文字
          <template #end><span class="text-xs text-oi-muted">{{ isPath ? '如 img' : '如 blog' }}</span></template>
        </UiMenuItem>
      </UiMenu>
    </div>
    <UiInput
      v-else
      :id="`${id}-input`"
      v-model="model"
      mono
      spellcheck="false"
      autocomplete="off"
      :aria-label="label"
      :invalid="issues.some(issue => issue.level === 'error')"
      :placeholder="isPath ? '/uploads/{year}/{month}' : '{filename}-{year}{month}{day}'"
    />
    <div class="flex flex-wrap items-center gap-2.5 text-xs">
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded text-[12.5px] font-[550] text-oi-accent-2 hover:underline focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
        @click="manual = !manual; editing = -1"
      >
        <i :class="manual ? 'ri-apps-2-line' : 'ri-code-line'" aria-hidden="true" />{{ manual ? '改用选择' : '直接编辑' }}
      </button>
      <span v-if="note && !manual" class="text-oi-muted">{{ note }}</span>
    </div>
    <ul v-if="issues.length" class="flex flex-col gap-1.5" aria-live="polite">
      <li v-for="(issue, index) in issues" :key="index" class="flex items-start gap-[7px] text-[12.5px] leading-normal text-oi-text">
        <i :class="[issueIcon(issue.level), 'text-[15px] leading-[1.3]']" aria-hidden="true" />
        <span :class="['min-w-0 flex-1', issue.level === 'error' && 'text-oi-danger']">
          <span class="sr-only">{{ issue.level === 'error' ? '错误：' : issue.level === 'warn' ? '提醒：' : '说明：' }}</span>{{ issue.message }}
        </span>
        <button v-if="issue.fix" type="button" class="flex-none rounded font-[550] text-oi-accent-2 hover:underline focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring" @click="applyFix(issue)">修正</button>
      </li>
      <li v-if="fixableCount > 1" class="mt-0.5">
        <UiButton size="sm" @click="fixAll"><i class="ri-magic-line" aria-hidden="true" />全部修正</UiButton>
      </li>
    </ul>
  </div>
</template>
