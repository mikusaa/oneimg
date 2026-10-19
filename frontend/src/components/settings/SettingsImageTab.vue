<script setup lang="ts">
import { toast, UiSwitch } from '@/components/ui'
import FormatChips from './FormatChips.vue'
import SettingRow from './SettingRow.vue'
import SettingSection from './SettingSection.vue'
import { IMAGE_TYPES, type SettingsDraft, type SettingsKey } from './settingsModel'

defineProps<{
  draft: SettingsDraft
  errors: Partial<Record<string, string>>
  dirty: (key: SettingsKey) => boolean
}>()
</script>

<template>
  <SettingSection title="主图保存" description="只作用于图片，视频始终保存原文件。">
    <SettingRow compact title="保存原图" description="主图保持上传时的字节、格式和扩展名，不压缩、不转换。" :dirty="dirty('original_image')">
      <UiSwitch v-model="draft.original_image" aria-label="保存原图" />
    </SettingRow>
    <div v-if="draft.original_image" class="flex items-center gap-2 bg-oi-warn-soft px-4 py-2.5 text-[12.5px] text-oi-warn min-[900px]:px-[18px]" role="note">
      <i class="ri-information-line" aria-hidden="true" />已开启保存原图，下面的 WebP、压缩质量和跳过格式暂不生效。
    </div>
    <SettingRow compact title="保存为 WebP" description="通常能明显减小体积。" :disabled="draft.original_image" :dirty="dirty('save_webp')">
      <UiSwitch v-model="draft.save_webp" aria-label="保存为 WebP" />
    </SettingRow>
    <SettingRow title="压缩质量" for="set-quality" description="0–100，默认 85。数值越高画质越好、体积越大。" :disabled="draft.original_image" :dirty="dirty('main_image_quality')">
      <div class="flex w-full flex-col gap-1.5">
        <div class="flex w-full items-center gap-3">
          <input
            v-model.number="draft.main_image_quality"
            type="range"
            min="0"
            max="100"
            step="1"
            class="min-w-0 flex-1 accent-[var(--oi-accent)]"
            aria-label="压缩质量滑块"
          >
          <input
            id="set-quality"
            v-model.number="draft.main_image_quality"
            type="number"
            min="0"
            max="100"
            step="1"
            inputmode="numeric"
            :aria-invalid="!!errors.main_image_quality || undefined"
            :class="['h-8 w-[60px] rounded-lg border bg-oi-raised px-2 text-right text-[13px] tabular-nums text-oi-ink outline-none focus:shadow-[0_0_0_3px_var(--oi-ring)]', errors.main_image_quality ? 'border-oi-danger' : 'border-oi-line-2 focus:border-oi-accent']"
          >
        </div>
        <p v-if="errors.main_image_quality" class="text-xs text-oi-danger" role="alert">{{ errors.main_image_quality }}</p>
      </div>
    </SettingRow>
    <SettingRow title="跳过压缩的格式" stack :disabled="draft.original_image" :dirty="dirty('skip_compress_formats')">
      <template #description>命中的图片保持原格式，不转 WebP、不压缩，适合动图。至少保留一种。</template>
      <FormatChips v-model="draft.skip_compress_formats" :options="IMAGE_TYPES" label="跳过压缩的格式" keep-one @blocked="toast.error('至少保留一种格式')" />
      <p v-if="errors.skip_compress_formats" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.skip_compress_formats }}</p>
    </SettingRow>
  </SettingSection>

  <SettingSection title="缩略图" description="图库列表优先加载缩略图，不影响对外的主图链接。">
    <SettingRow compact title="生成缩略图" :dirty="dirty('thumbnail')">
      <UiSwitch v-model="draft.thumbnail" aria-label="生成缩略图" />
    </SettingRow>
  </SettingSection>
</template>
