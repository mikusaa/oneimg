<script setup lang="ts">
import { SwitchRoot, SwitchThumb } from 'reka-ui'

defineProps<{ disabled?: boolean }>()

const model = defineModel<boolean>({ default: false })
</script>

<template>
  <!-- 没有可见文字时请传 aria-label -->
  <SwitchRoot
    v-model="model"
    :disabled="disabled"
    class="relative inline-flex h-[21px] w-9 shrink-0 cursor-pointer items-center rounded-full bg-oi-line-2 transition-colors duration-200 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-oi-accent"
  >
    <SwitchThumb
      class="block h-4 w-4 translate-x-[2.5px] rounded-full bg-white shadow-[0_1px_3px_rgba(0,0,0,0.25)] transition-transform duration-200 ease-[cubic-bezier(0.3,0.7,0.4,1.2)] data-[state=checked]:translate-x-[17.5px] motion-reduce:transition-none"
    />
  </SwitchRoot>
</template>
