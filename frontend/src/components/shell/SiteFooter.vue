<script setup lang="ts">
import { computed } from 'vue'
import { PRODUCT_NAME, useSite } from '@/composables/useSite'
import securityImg from '@/assets/images/gongan.png'
import packageInfo from '../../../package.json'

// 页脚：版权、OneImg 版本（链接到对应 Release）、ICP 和公安备案号
const site = useSite()
const year = new Date().getFullYear()
const version = packageInfo.version
const releaseUrl = `https://github.com/mikusaa/oneimg/releases/tag/v${version}`
const versionLabel = computed(() => site.title === PRODUCT_NAME ? `v${version}` : `${PRODUCT_NAME} v${version}`)
</script>

<template>
  <div class="px-2 text-[11px] leading-[1.6] text-oi-faint [&_a:hover]:text-oi-accent-2 [&_a]:text-oi-muted [&_a]:transition-colors">
    <div>
      © {{ year }} {{ site.title }} ·
      <a :href="releaseUrl" target="_blank" rel="noopener noreferrer">{{ versionLabel }}</a>
    </div>
    <div v-if="site.icp || site.publicSecurity" class="mt-0.5 flex flex-wrap gap-x-2.5 gap-y-1">
      <a v-if="site.icp" href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-[3px]">
        <i class="ri-government-line" aria-hidden="true" />{{ site.icp }}
      </a>
      <a v-if="site.publicSecurity" href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-[3px]">
        <img :src="securityImg" alt="" class="h-3 w-3" />{{ site.publicSecurity }}
      </a>
    </div>
  </div>
</template>
