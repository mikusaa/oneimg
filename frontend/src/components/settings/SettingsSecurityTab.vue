<script setup lang="ts">
import { UiInput, UiSwitch } from '@/components/ui'
import SettingRow from './SettingRow.vue'
import SettingSection from './SettingSection.vue'
import type { SettingsDraft, SettingsKey } from './settingsModel'

defineProps<{
  draft: SettingsDraft
  errors: Partial<Record<string, string>>
  dirty: (key: SettingsKey) => boolean
  /** 白名单的生效范围；读不到存储列表时为 null */
  scope: { proxied: string[]; direct: string[] } | null
}>()
</script>

<template>
  <SettingSection title="注册">
    <template #description>个人密码、Passkey 和 API 令牌在 <RouterLink to="/account" class="text-oi-accent-2 hover:underline">账户设置</RouterLink> 中管理。</template>
    <SettingRow compact title="开放用户注册" description="新账号固定为普通用户。关闭后只能由管理员创建账号。" :dirty="dirty('registration_enabled')">
      <UiSwitch v-model="draft.registration_enabled" aria-label="开放用户注册" />
    </SettingRow>
  </SettingSection>

  <SettingSection title="防盗链" description="只允许指定网站嵌入图片。空 Referer（直接打开链接）和本站始终允许。">
    <SettingRow compact title="启用来源白名单" :dirty="dirty('referer_white_enable')">
      <UiSwitch v-model="draft.referer_white_enable" aria-label="启用来源白名单" />
    </SettingRow>
    <SettingRow title="允许的来源域名" for="set-referer" description="英文逗号分隔，自动包含子域名。" stack :disabled="!draft.referer_white_enable" :dirty="dirty('referer_white_list')">
      <UiInput id="set-referer" v-model="draft.referer_white_list" multiline mono rows="3" placeholder="example.com,blog.example.com" spellcheck="false" />
    </SettingRow>
    <SettingRow stack>
      <template #title><span class="text-[12.5px]">生效范围</span></template>
      <template #description>
        <template v-if="scope">白名单只对经 OneImg 访问的链接生效；设置了直链或 CDN 域名的存储，浏览器直接访问存储，不受白名单保护。</template>
        <template v-else>白名单只对经 OneImg 访问的链接生效；设置了直链或 CDN 域名的存储不受保护，具体见 <RouterLink to="/storage" class="text-oi-accent-2 hover:underline">存储</RouterLink> 页每张卡片的「访问地址」。</template>
      </template>
      <div v-if="scope" class="flex flex-wrap gap-1.5">
        <span v-for="name in scope.proxied" :key="`p-${name}`" class="inline-flex h-6 items-center gap-1 rounded-full bg-oi-accent-soft px-2 text-xs font-[550] text-oi-accent-2">
          <i class="ri-shield-check-line text-[13px]" aria-hidden="true" />{{ name }}
        </span>
        <span v-for="name in scope.direct" :key="`d-${name}`" class="inline-flex h-6 items-center gap-1 rounded-full bg-oi-panel px-2 text-xs font-[550] text-oi-muted" title="已配置 CDN / 直链域名，浏览器不经过 OneImg">
          <i class="ri-forbid-line text-[13px]" aria-hidden="true" />{{ name }} · 直连不生效
        </span>
      </div>
    </SettingRow>
  </SettingSection>
</template>
