<script setup lang="ts">
import { computed, ref, useAttrs } from 'vue'
import UiInput from './UiInput.vue'

// class / style 放在外层，其余属性（id、autocomplete、placeholder…）给输入框
defineOptions({ inheritAttrs: false })
const attrs = useAttrs()
const inputAttrs = computed(() => {
  const { class: _class, style: _style, ...rest } = attrs
  return rest
})
withDefaults(defineProps<{
  invalid?: boolean
  /** 显示「显示 / 隐藏密码」按钮 */
  revealable?: boolean
}>(), { invalid: false, revealable: true })

const model = defineModel<string>({ default: '' })
const shown = ref(false)
</script>

<template>
  <div class="relative" :class="attrs.class" :style="attrs.style as string">
    <UiInput
      v-model="model"
      v-bind="inputAttrs"
      :type="shown ? 'text' : 'password'"
      :invalid="invalid"
      :class="revealable && 'pr-10'"
      spellcheck="false"
    />
    <button
      v-if="revealable"
      type="button"
      class="absolute right-[3px] top-[3px] grid h-[30px] w-[30px] place-items-center rounded-lg text-base text-oi-muted transition-colors hover:bg-oi-hover hover:text-oi-ink focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-oi-ring"
      :aria-label="shown ? '隐藏密码' : '显示密码'"
      :title="shown ? '隐藏密码' : '显示密码'"
      :aria-pressed="shown"
      @click="shown = !shown"
    >
      <i :class="shown ? 'ri-eye-off-line' : 'ri-eye-line'" aria-hidden="true" />
    </button>
  </div>
</template>
