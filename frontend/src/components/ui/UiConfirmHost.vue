<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import UiButton from './UiButton.vue'
import UiDialog from './UiDialog.vue'
import UiField from './UiField.vue'
import UiInput from './UiInput.vue'
import { confirmState, settleConfirm } from './confirm'

const typed = ref('')
const options = computed(() => confirmState.options)
const canConfirm = computed(() => !options.value?.requireText || typed.value.trim() === options.value.requireText)

watch(() => confirmState.open, open => { if (open) typed.value = '' })

// Esc、点遮罩等关闭方式都按取消处理
const onOpenChange = (open: boolean) => { if (!open && confirmState.resolve) settleConfirm(false) }
</script>

<template>
  <!-- 整个应用只挂载一次 -->
  <UiDialog
    v-if="options"
    :open="confirmState.open"
    :title="options.title"
    :description="options.description"
    :icon="options.icon || (options.danger ? 'ri-error-warning-line' : 'ri-question-line')"
    :tone="options.danger ? 'danger' : 'default'"
    @update:open="onOpenChange"
  >
    <template v-if="options.points?.length" #header>
      <ul
        :class="['mt-2.5 list-disc pl-[18px] text-[13px] leading-relaxed text-oi-text', options.danger ? 'marker:text-oi-danger' : 'marker:text-oi-faint']"
      >
        <li v-for="point in options.points" :key="point">{{ point }}</li>
      </ul>
    </template>
    <template v-if="options.requireText">
      <UiField :label="`输入「${options.requireText}」确认`" for="ui-confirm-text">
        <UiInput id="ui-confirm-text" v-model="typed" autocomplete="off" @keydown.enter.prevent="canConfirm && settleConfirm(true)" />
      </UiField>
    </template>
    <template #footer>
      <UiButton variant="ghost" @click="settleConfirm(false)">{{ options.cancelText || '取消' }}</UiButton>
      <UiButton :variant="options.danger ? 'danger-solid' : 'primary'" :disabled="!canConfirm" @click="settleConfirm(true)">
        {{ options.confirmText || '确定' }}
      </UiButton>
    </template>
  </UiDialog>
</template>
