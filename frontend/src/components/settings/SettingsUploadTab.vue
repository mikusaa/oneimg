<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { toast, UiSwitch } from '@/components/ui'
import FormatChips from './FormatChips.vue'
import MegabyteInput from './MegabyteInput.vue'
import SettingRow from './SettingRow.vue'
import SettingSection from './SettingSection.vue'
import TemplateBuilder from './TemplateBuilder.vue'
import { IMAGE_TYPES, type SettingsDraft, type SettingsKey } from './settingsModel'
import { DEFAULT_PATH, fillTemplate, isRandomName, objectDir, outputName, PATH_PRESETS } from './template'

export interface SampleStorage {
  name: string
  type: string
  /** 直连域名；空表示经 OneImg */
  domain: string
  /** 本地 CDN 链接去掉 /uploads 前缀 */
  stripUploads: boolean
}

const props = defineProps<{
  draft: SettingsDraft
  errors: Partial<Record<string, string>>
  dirty: (key: SettingsKey) => boolean
  /** 默认存储，用来计算示例链接 */
  sampleStorage: SampleStorage | null
}>()

/* ---------- 存储路径：常用方案，其余归为「自定义」 ---------- */
// 旧安装的默认值是 /uploads/{year}/{moon}，{moon} 等同于 {month}，也算「按年 / 月」
const normalizePath = (value: string) => `/${value.trim().replace(/^\/+|\/+$/g, '').split('{moon}').join('{month}')}`
const customPath = ref(false)
const presetKey = computed(() => PATH_PRESETS.find(preset => normalizePath(preset.value) === normalizePath(props.draft.default_path || DEFAULT_PATH))?.key)
const pathMode = computed(() => customPath.value ? 'custom' : presetKey.value ?? 'custom')
const pickPath = (value: string | null) => {
  customPath.value = value === null
  if (value !== null) props.draft.default_path = value
}
// 放弃更改后回到方案选择
watch(() => props.dirty('default_path'), dirty => { if (!dirty && presetKey.value) customPath.value = false })

/* ---------- 文件名：随机名 / 原文件名 / 自定义，对应 file_name 和 save_original_name ---------- */
const lastCustom = ref(isRandomName(props.draft.file_name) ? '{filename}-{year}{month}{day}' : props.draft.file_name)
const nameMode = computed(() => props.draft.save_original_name ? 'original' : isRandomName(props.draft.file_name) ? 'random' : 'custom')
const pickName = (mode: string) => {
  if (nameMode.value === 'custom') lastCustom.value = props.draft.file_name
  props.draft.save_original_name = mode === 'original'
  if (mode === 'random') props.draft.file_name = '{random}'
  if (mode === 'custom' && isRandomName(props.draft.file_name)) props.draft.file_name = lastCustom.value
}
const nameModes = computed(() => [
  { key: 'random', label: '随机名', tag: '推荐', sample: outputName({ keepName: false, nameTemplate: '{random}' }, 'hero-banner.png', '.webp') },
  { key: 'original', label: '原文件名', tag: '', sample: outputName({ keepName: true, nameTemplate: '' }, 'hero-banner.png', '.webp') },
  { key: 'custom', label: '自定义', tag: '', sample: outputName({ keepName: false, nameTemplate: nameMode.value === 'custom' ? props.draft.file_name : lastCustom.value }, 'hero-banner.png', '.webp') },
])

/* ---------- 示例 ---------- */
const samplePath = computed(() => {
  const dir = objectDir(fillTemplate(props.draft.default_path.trim() || DEFAULT_PATH), props.sampleStorage?.type)
  return `/${dir ? `${dir}/` : ''}${outputName({ keepName: props.draft.save_original_name, nameTemplate: props.draft.file_name }, 'hero-banner.png', '.webp')}`
})
const sampleLink = computed(() => {
  const storage = props.sampleStorage
  const path = samplePath.value
  if (!storage?.domain) return window.location.origin + path
  // 本地 CDN 只对 uploads/ 下的文件去掉前缀
  if (storage.stripUploads) return path.startsWith('/uploads/') ? storage.domain + path.slice('/uploads'.length) : window.location.origin + path
  return storage.domain + path
})

const choice = 'flex min-w-0 cursor-pointer items-start gap-2.5 rounded-xl border px-3 py-[11px] transition-colors hover:bg-oi-hover has-[:focus-visible]:shadow-[0_0_0_3px_var(--oi-ring)]'
const choiceOn = 'border-oi-accent bg-oi-accent-soft hover:bg-oi-accent-soft'
</script>

<template>
  <SettingSection title="允许上传的文件" description="不符合规则的文件会在待上传列表中直接标记失败。">
    <SettingRow title="图片类型" stack :dirty="dirty('allowed_types')">
      <template #description>按文件内容判断，至少保留一种。只支持 JPEG、PNG、GIF、WebP；SVG（可能带脚本）和其他格式一律不接受，以前上传的 SVG 仍可访问。</template>
      <FormatChips v-model="draft.allowed_types" :options="IMAGE_TYPES" label="允许的图片类型" keep-one @blocked="toast.error('至少保留一种图片类型')" />
      <p v-if="errors.allowed_types" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.allowed_types }}</p>
    </SettingRow>
    <SettingRow title="图片大小上限" for="set-max-file-size" description="默认 10 MB" :dirty="dirty('max_file_size')">
      <div class="flex flex-col items-start gap-1.5 min-[900px]:items-end">
        <MegabyteInput id="set-max-file-size" v-model="draft.max_file_size" :invalid="!!errors.max_file_size" :describedby="errors.max_file_size ? 'set-max-file-size-error' : undefined" />
        <p v-if="errors.max_file_size" id="set-max-file-size-error" class="text-xs text-oi-danger" role="alert">{{ errors.max_file_size }}</p>
      </div>
    </SettingRow>
    <SettingRow compact title="允许上传视频" description="MP4 / WebM 原文件保存，不压缩不转码。关闭后已有视频仍可播放。" :dirty="dirty('video_upload_enabled')">
      <UiSwitch v-model="draft.video_upload_enabled" aria-label="允许上传视频" />
    </SettingRow>
    <SettingRow title="视频大小上限" for="set-max-video" description="1 MB – 1024 MB，默认 100 MB" :disabled="!draft.video_upload_enabled" :dirty="dirty('max_video_file_size')">
      <div class="flex flex-col items-start gap-1.5 min-[900px]:items-end">
        <MegabyteInput id="set-max-video" v-model="draft.max_video_file_size" :min="1" :invalid="!!errors.max_video_file_size" :describedby="errors.max_video_file_size ? 'set-max-video-error' : undefined" />
        <p v-if="errors.max_video_file_size" id="set-max-video-error" class="text-xs text-oi-danger" role="alert">{{ errors.max_video_file_size }}</p>
      </div>
    </SettingRow>
  </SettingSection>

  <SettingSection title="存储路径与文件名" description="决定文件在存储中的位置，也就是链接里域名之后的部分。">
    <SettingRow title="存储路径" stack :dirty="dirty('default_path')">
      <template #description>文件放在存储里的哪个目录。只影响新上传的文件，已有文件的位置和链接不变。</template>
      <div class="flex flex-col gap-2.5">
        <div class="grid grid-cols-1 gap-2 min-[560px]:grid-cols-[repeat(auto-fill,minmax(210px,1fr))]" role="radiogroup" aria-label="存储路径方案">
          <label v-for="preset in PATH_PRESETS" :key="preset.key" :class="[choice, pathMode === preset.key ? choiceOn : 'border-oi-line-2']">
            <input type="radio" name="path-mode" class="mt-[3px] h-[15px] w-[15px] flex-none accent-[var(--oi-accent)]" :checked="pathMode === preset.key" @change="pickPath(preset.value)">
            <span class="min-w-0">
              <b class="flex items-center gap-1.5 text-[13.5px] font-semibold text-oi-ink">{{ preset.label }}<span v-if="preset.tag" class="inline-flex h-[18px] items-center rounded-full bg-oi-panel px-1.5 text-[11px] font-[550] text-oi-muted">{{ preset.tag }}</span></b>
              <code class="mt-[3px] block break-all font-oi-mono text-[11.5px] text-oi-text">{{ preset.value }}</code>
              <em v-if="preset.desc" class="mt-0.5 block text-[12.5px] not-italic leading-normal text-oi-muted">{{ preset.desc }}</em>
            </span>
          </label>
          <label :class="[choice, pathMode === 'custom' ? choiceOn : 'border-oi-line-2']">
            <input type="radio" name="path-mode" class="mt-[3px] h-[15px] w-[15px] flex-none accent-[var(--oi-accent)]" :checked="pathMode === 'custom'" @change="pickPath(null)">
            <span class="min-w-0">
              <b class="text-[13.5px] font-semibold text-oi-ink">自定义</b>
              <em class="mt-0.5 block text-[12.5px] not-italic leading-normal text-oi-muted">自己选择按什么分目录，也可以直接编辑</em>
            </span>
          </label>
        </div>
        <TemplateBuilder v-if="pathMode === 'custom'" id="set-path" v-model="draft.default_path" kind="path" />
        <p v-if="errors.default_path && pathMode !== 'custom'" class="text-xs text-oi-danger" role="alert">{{ errors.default_path }}</p>
      </div>
    </SettingRow>

    <SettingRow title="文件名" stack :dirty="dirty('file_name') || dirty('save_original_name')">
      <template #description>除「随机名」外，名字后面都会自动加一段随机 ID，避免重名。扩展名按最终保存的格式。</template>
      <div class="flex flex-col gap-2.5">
        <div class="grid grid-cols-1 gap-2 min-[560px]:grid-cols-3" role="radiogroup" aria-label="文件名方式">
          <label v-for="mode in nameModes" :key="mode.key" :class="[choice, nameMode === mode.key ? choiceOn : 'border-oi-line-2']">
            <input type="radio" name="name-mode" class="mt-[3px] h-[15px] w-[15px] flex-none accent-[var(--oi-accent)]" :checked="nameMode === mode.key" @change="pickName(mode.key)">
            <span class="min-w-0">
              <b class="flex items-center gap-1.5 text-[13.5px] font-semibold text-oi-ink">{{ mode.label }}<span v-if="mode.tag" class="inline-flex h-[18px] items-center rounded-full bg-oi-panel px-1.5 text-[11px] font-[550] text-oi-muted">{{ mode.tag }}</span></b>
              <code class="mt-[3px] block break-all font-oi-mono text-[11.5px] text-oi-text">{{ mode.sample }}</code>
            </span>
          </label>
        </div>
        <TemplateBuilder v-if="nameMode === 'custom'" id="set-name" v-model="draft.file_name" kind="name" />
      </div>
    </SettingRow>

    <div class="flex flex-col gap-1.5 rounded-b-oi-lg bg-oi-panel px-4 py-3.5 min-[900px]:px-[18px]">
      <div class="flex min-w-0 items-baseline gap-3">
        <span class="w-[84px] flex-none text-[12.5px] font-[560] text-oi-ink">文件位置</span>
        <code class="min-w-0 break-all font-oi-mono text-[12.5px] text-oi-ink">{{ samplePath }}</code>
      </div>
      <div class="flex min-w-0 items-baseline gap-3">
        <span class="w-[84px] flex-none text-[12.5px] font-[560] text-oi-ink">默认存储链接</span>
        <code class="min-w-0 break-all font-oi-mono text-[12.5px] text-oi-ink">{{ sampleLink }}</code>
      </div>
      <p class="text-[12.5px] leading-normal text-oi-muted">
        示例按上传「hero-banner.png」、保存为 WebP 计算<template v-if="sampleStorage">；链接按默认存储「{{ sampleStorage.name }}」的访问地址生成</template>。
      </p>
    </div>
  </SettingSection>
</template>
