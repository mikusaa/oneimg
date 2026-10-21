<script setup lang="ts">
import { computed, ref } from 'vue'
import { toast, UiSegmented, UiSwitch } from '@/components/ui'
import FormatChips from './FormatChips.vue'
import SettingRow from './SettingRow.vue'
import SettingSection from './SettingSection.vue'
import { IMAGE_TYPES, type SettingsDraft, type SettingsKey } from './settingsModel'

const props = defineProps<{
  draft: SettingsDraft
  errors: Partial<Record<string, string>>
  dirty: (key: SettingsKey) => boolean
}>()

// 压缩质量用几档预设代替 0–100 的数值；不在预设里的旧值显示为「自定义」
const QUALITY_PRESETS = [
  { value: 95, label: '画质优先', hint: '细节保留最多，体积较大。' },
  { value: 85, label: '均衡', hint: '默认，画质和体积兼顾，适合大多数图片。' },
  { value: 70, label: '体积优先', hint: '体积更小，细节和渐变处可能看出压缩痕迹。' },
]
const qualityOptions = [...QUALITY_PRESETS.map(preset => ({ value: String(preset.value), label: preset.label })), { value: 'custom', label: '自定义' }]
const presetFor = (quality: number) => QUALITY_PRESETS.find(preset => preset.value === quality)
const customQuality = ref(!presetFor(props.draft.main_image_quality))
const qualityMode = computed({
  get: () => customQuality.value || !presetFor(props.draft.main_image_quality) ? 'custom' : String(props.draft.main_image_quality),
  set: (mode: string | undefined) => {
    customQuality.value = mode === 'custom'
    if (mode && mode !== 'custom') props.draft.main_image_quality = Number(mode)
  },
})
const qualityHint = computed(() => qualityMode.value === 'custom'
  ? '0–100 的整数，数值越高画质越好、体积越大。'
  : presetFor(props.draft.main_image_quality)?.hint)

// 后端默认值里带 SVG，但新增 SVG 已不允许上传；照常保存，只是不显示
const HIDDEN_SKIP_FORMATS = ['image/svg+xml']
</script>

<template>
  <SettingSection title="主图保存" description="只作用于图片，视频始终保存原文件。">
    <SettingRow compact title="保存原图" description="主图保持上传时的字节、格式和扩展名，不压缩、不转换。" :dirty="dirty('original_image')">
      <UiSwitch v-model="draft.original_image" aria-label="保存原图" />
    </SettingRow>
    <div v-if="draft.original_image" class="flex items-center gap-2 bg-oi-warn-soft px-4 py-2.5 text-[12.5px] text-oi-warn min-[900px]:px-[18px]" role="note">
      <i class="ri-information-line" aria-hidden="true" />已开启保存原图，下面的 WebP、压缩质量和跳过格式暂不生效。
    </div>
    <SettingRow compact title="保存为 WebP" description="通常能明显减小体积；转换后没有明显变小时保留原文件。" :disabled="draft.original_image" :dirty="dirty('save_webp')">
      <UiSwitch v-model="draft.save_webp" aria-label="保存为 WebP" />
    </SettingRow>
    <SettingRow
      title="压缩质量"
      :description="draft.save_webp ? '转为 WebP 时使用。' : '只在开启「保存为 WebP」后生效。'"
      stack
      :disabled="draft.original_image || !draft.save_webp"
      :dirty="dirty('main_image_quality')"
    >
      <div class="flex flex-col gap-2">
        <div class="flex flex-wrap items-center gap-3">
          <UiSegmented v-model="qualityMode" :options="qualityOptions" aria-label="压缩质量" />
          <span v-if="qualityMode === 'custom'" class="flex items-center gap-2">
            <input
              id="set-quality"
              v-model.number="draft.main_image_quality"
              type="number"
              min="0"
              max="100"
              step="1"
              inputmode="numeric"
              aria-label="自定义压缩质量"
              :aria-invalid="!!errors.main_image_quality || undefined"
              :class="['h-8 w-[64px] rounded-lg border bg-oi-raised px-2 text-right text-[13px] tabular-nums text-oi-ink outline-none focus:shadow-[0_0_0_3px_var(--oi-ring)]', errors.main_image_quality ? 'border-oi-danger' : 'border-oi-line-2 focus:border-oi-accent']"
            >
            <span class="text-xs text-oi-muted">/ 100</span>
          </span>
        </div>
        <p class="text-xs text-oi-muted">
          <span v-if="qualityMode !== 'custom'" class="tabular-nums">质量 {{ draft.main_image_quality }} · </span>{{ qualityHint }}
        </p>
        <p v-if="errors.main_image_quality" class="text-xs text-oi-danger" role="alert">{{ errors.main_image_quality }}</p>
      </div>
    </SettingRow>
    <SettingRow title="跳过压缩的格式" stack :disabled="draft.original_image" :dirty="dirty('skip_compress_formats')">
      <template #description>命中的图片保持原格式，不转 WebP、不压缩，适合动图。至少保留一种。</template>
      <FormatChips v-model="draft.skip_compress_formats" :options="IMAGE_TYPES" :hidden="HIDDEN_SKIP_FORMATS" label="跳过压缩的格式" keep-one @blocked="toast.error('至少保留一种格式')" />
      <p v-if="errors.skip_compress_formats" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.skip_compress_formats }}</p>
    </SettingRow>
  </SettingSection>

  <SettingSection title="缩略图" description="图库列表优先加载缩略图，不影响对外的主图链接。">
    <SettingRow compact title="生成缩略图" :dirty="dirty('thumbnail')">
      <UiSwitch v-model="draft.thumbnail" aria-label="生成缩略图" />
    </SettingRow>
  </SettingSection>
</template>
