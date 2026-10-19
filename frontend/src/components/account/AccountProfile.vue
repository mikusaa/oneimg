<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, ApiError, errorMessage } from '@/api/client.ts'
import { passwordError, usernameError } from '@/components/auth/auth'
import SettingRow from '@/components/settings/SettingRow.vue'
import SettingSection from '@/components/settings/SettingSection.vue'
import { confirm, toast, UiButton, UiInput, UiPasswordInput } from '@/components/ui'
import { clearSession } from '@/composables/useSession'
import { hasPendingUploads } from '@/composables/useUploadQueue'

const props = defineProps<{
  username: string
  /** 只有管理员能改自己的用户名 */
  canRename: boolean
  /** 首次登录强制改密：只能改密码 */
  mustChange: boolean
  /** 旧默认密码恢复：还需要服务器上的一次性校验码 */
  bootstrap: boolean
}>()

const router = useRouter()
const form = reactive({ username: '', password: '', confirm: '', code: '', current: '' })
const errors = reactive<Record<string, string>>({})
const formError = ref('')
const saving = ref(false)

const validate = () => {
  for (const key of Object.keys(errors)) delete errors[key]
  const renaming = props.canRename && !props.mustChange && form.username.trim() !== '' && form.username.trim() !== props.username
  const changingPassword = form.password !== ''
  if (props.mustChange && !changingPassword) errors.password = '请设置新密码'
  else if (!renaming && !changingPassword) formError.value = '没有需要保存的修改：填写新用户名或新密码'
  if (renaming) {
    const message = usernameError(form.username)
    if (message) errors.username = message
  }
  if (changingPassword) {
    const message = passwordError(form.password)
    if (message) errors.password = message
    else if (form.password === '123456') errors.password = '不能使用默认密码 123456'
    else if (form.password === form.current) errors.password = '新密码不能和当前密码相同'
    else if (form.password !== form.confirm) errors.confirm = '两次输入的新密码不一致'
  }
  if (props.bootstrap && !form.code.trim()) errors.code = '请输入一次性校验码'
  if (!form.current) errors.current = '请输入当前密码'
  return { ok: !Object.keys(errors).length && !formError.value, renaming, changingPassword }
}

// 服务器的错误码对应到具体字段
const applyServerError = (error: unknown) => {
  const code = error instanceof ApiError ? error.problem?.code : ''
  const message = errorMessage(error, '保存失败，请稍后重试')
  if (code === 'current_password_invalid') errors.current = message
  else if (code === 'invalid_bootstrap_code') errors.code = message
  else if (code === 'username_conflict' || code === 'username_change_forbidden') errors.username = message
  else if (code === 'validation_error' && message.includes('用户名')) errors.username = message
  else if (code === 'validation_error' && message.includes('密码')) errors.password = '新密码无效：需要 6–100 个字符，不能是 123456，也不能和当前密码相同'
  else formError.value = message
}

const save = async () => {
  if (saving.value) return
  formError.value = ''
  const { ok, renaming, changingPassword } = validate()
  if (!ok) return
  // 保存后会退出登录，上传队列会被清空
  if (hasPendingUploads() && !await confirm({
    title: '保存并重新登录？',
    description: '保存后需要重新登录，待上传列表里的文件会被清空，正在上传的文件可能没有保存。',
    danger: true,
    confirmText: '保存并退出',
  })) return
  saving.value = true
  try {
    await api.updateMe({
      current_password: form.current,
      username: renaming ? form.username.trim() : undefined,
      password: changingPassword ? form.password : undefined,
      bootstrap_code: props.bootstrap ? form.code.trim() : undefined,
    })
    const wasRestricted = props.mustChange
    clearSession()
    toast.success(wasRestricted ? '密码已修改，请用新密码登录' : '已保存，请重新登录')
    await router.replace('/login')
  } catch (error) {
    applyServerError(error)
    form.current = ''
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <SettingSection id="account-profile" title="资料与密码" description="保存后需要重新登录。">
    <form class="divide-y divide-oi-line" novalidate @submit.prevent="save">
      <SettingRow
        v-if="!mustChange"
        title="用户名"
        for="account-username"
        :description="canRename ? '留空则不修改，3–50 个字符' : '普通用户的用户名需要由管理员修改'"
      >
        <div class="w-full">
          <UiInput
            id="account-username"
            v-model="form.username"
            :placeholder="username"
            maxlength="50"
            autocomplete="username"
            :disabled="!canRename"
            :invalid="!!errors.username"
            :aria-describedby="errors.username ? 'account-username-error' : undefined"
          />
          <p v-if="errors.username" id="account-username-error" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.username }}</p>
        </div>
      </SettingRow>
      <SettingRow title="新密码" for="account-password" :description="mustChange ? '必填，至少 6 位' : '留空则不修改，至少 6 位'">
        <div class="w-full">
          <UiPasswordInput
            id="account-password"
            v-model="form.password"
            maxlength="100"
            autocomplete="new-password"
            :invalid="!!errors.password"
            :aria-describedby="errors.password ? 'account-password-error' : undefined"
          />
          <p v-if="errors.password" id="account-password-error" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.password }}</p>
        </div>
      </SettingRow>
      <SettingRow title="确认新密码" for="account-confirm" :disabled="!form.password && !mustChange">
        <div class="w-full">
          <UiPasswordInput
            id="account-confirm"
            v-model="form.confirm"
            maxlength="100"
            autocomplete="new-password"
            :revealable="false"
            :invalid="!!errors.confirm"
            :aria-describedby="errors.confirm ? 'account-confirm-error' : undefined"
          />
          <p v-if="errors.confirm" id="account-confirm-error" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.confirm }}</p>
        </div>
      </SettingRow>
      <SettingRow v-if="bootstrap" title="服务器本地一次性校验码" for="account-code">
        <template #description>位于服务器数据目录中的 <span class="font-oi-mono">.admin_recovery_code</span> 文件</template>
        <div class="w-full">
          <UiPasswordInput
            id="account-code"
            v-model="form.code"
            mono
            autocomplete="off"
            :invalid="!!errors.code"
            :aria-describedby="errors.code ? 'account-code-error' : undefined"
          />
          <p v-if="errors.code" id="account-code-error" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.code }}</p>
        </div>
      </SettingRow>
      <SettingRow title="当前密码" for="account-current" description="任何修改都需要验证当前密码">
        <div class="w-full">
          <UiPasswordInput
            id="account-current"
            v-model="form.current"
            autocomplete="current-password"
            :invalid="!!errors.current"
            :aria-describedby="errors.current ? 'account-current-error' : undefined"
          />
          <p v-if="errors.current" id="account-current-error" class="mt-1.5 text-xs text-oi-danger" role="alert">{{ errors.current }}</p>
        </div>
      </SettingRow>
      <div class="flex flex-wrap items-center justify-end gap-3 px-4 py-3.5 min-[900px]:px-[18px]">
        <p v-if="formError" class="mr-auto flex items-center gap-1.5 text-[12.5px] text-oi-danger" role="alert">
          <i class="ri-error-warning-line" aria-hidden="true" />{{ formError }}
        </p>
        <UiButton variant="primary" type="submit" :disabled="saving">
          <i v-if="saving" class="ri-loader-4-line animate-spin" aria-hidden="true" />{{ mustChange ? '修改密码' : '保存修改' }}
        </UiButton>
      </div>
    </form>
  </SettingSection>
</template>
