<script setup lang="ts">
import { ref, watch } from 'vue'
import { UiInput } from '@/components/ui'
import SettingRow from './SettingRow.vue'
import SettingSection from './SettingSection.vue'
import type { SettingsDraft, SettingsKey } from './settingsModel'

const props = defineProps<{
  draft: SettingsDraft
  errors: Partial<Record<string, string>>
  dirty: (key: SettingsKey) => boolean
}>()

// 图标地址打不开时显示占位
const iconBroken = ref(false)
watch(() => props.draft.seo_icon, () => { iconBroken.value = false })
</script>

<template>
  <SettingSection title="基本信息" description="显示在浏览器标题、登录页和搜索结果中。">
    <SettingRow title="网站标题" for="set-title" description="留空时显示 OneImg；改过名称时，侧栏品牌下方会注明 OneImg" :dirty="dirty('seo_title')">
      <UiInput id="set-title" v-model="draft.seo_title" maxlength="100" placeholder="OneImg" />
    </SettingRow>
    <SettingRow title="网站描述" for="set-description" :dirty="dirty('seo_description')">
      <UiInput id="set-description" v-model="draft.seo_description" maxlength="300" />
    </SettingRow>
    <SettingRow title="关键词" for="set-keywords" description="英文逗号分隔" :dirty="dirty('seo_keywords')">
      <UiInput id="set-keywords" v-model="draft.seo_keywords" maxlength="300" />
    </SettingRow>
    <SettingRow title="网站图标" for="set-icon" description="填写图标地址，也可以先上传到图库再粘贴链接。设置后代替 OneImg Logo" :dirty="dirty('seo_icon')">
      <div class="flex w-full items-center gap-2">
        <span class="grid h-9 w-9 flex-none place-items-center overflow-hidden rounded-[9px] border border-oi-line bg-oi-panel text-oi-faint" aria-hidden="true">
          <img v-if="draft.seo_icon && !iconBroken" :src="draft.seo_icon" alt="" class="h-[22px] w-[22px] object-contain" @error="iconBroken = true">
          <i v-else :class="iconBroken ? 'ri-error-warning-line text-oi-warn' : 'ri-image-line'" />
        </span>
        <UiInput id="set-icon" v-model="draft.seo_icon" mono spellcheck="false" placeholder="https://example.com/favicon.ico" :aria-describedby="iconBroken ? 'set-icon-broken' : undefined" />
      </div>
      <p v-if="iconBroken" id="set-icon-broken" class="sr-only">图标地址无法打开</p>
    </SettingRow>
  </SettingSection>

  <SettingSection title="备案" description="显示在页脚，留空则不显示。">
    <SettingRow title="ICP 备案号" for="set-icp" :dirty="dirty('seo_icp')">
      <UiInput id="set-icp" v-model="draft.seo_icp" placeholder="京ICP备 00000000 号" />
    </SettingRow>
    <SettingRow title="公安备案号" for="set-police" :dirty="dirty('public_security')">
      <UiInput id="set-police" v-model="draft.public_security" placeholder="京公网安备 00000000000000 号" />
    </SettingRow>
  </SettingSection>
</template>
