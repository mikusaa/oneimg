<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api, errorMessage, fieldErrors } from '@/api/client.ts'
import AuthCard from '@/components/auth/AuthCard.vue'
import { passwordError, usernameError } from '@/components/auth/auth'
import { toast, UiButton, UiField, UiInput, UiPasswordInput } from '@/components/ui'

const router = useRouter()
const form = reactive({ username: '', password: '', confirm: '' })
const errors = reactive<Record<string, string>>({})
const formError = ref('')
const busy = ref(false)
const ready = ref(false)

const validate = () => {
  for (const key of Object.keys(errors)) delete errors[key]
  const name = usernameError(form.username)
  const pwd = passwordError(form.password)
  if (name) errors.username = name
  if (pwd) errors.password = pwd
  else if (form.password !== form.confirm) errors.confirm = '两次输入的密码不一致'
  return !Object.keys(errors).length
}

const submit = async () => {
  if (busy.value) return
  formError.value = ''
  if (!validate()) return
  busy.value = true
  try {
    const username = form.username.trim()
    await api.register({ username, password: form.password })
    toast.success('注册成功，请登录')
    await router.replace({ path: '/login', query: { username } })
  } catch (error) {
    const fields = fieldErrors(error)
    Object.assign(errors, fields)
    if (!Object.keys(fields).length) formError.value = errorMessage(error, '注册失败，请稍后重试')
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  try {
    const config = await api.getPublicConfig()
    if (config.registration_enabled !== true) {
      toast.info('暂未开放注册，请联系管理员创建账号')
      await router.replace('/login')
      return
    }
    ready.value = true
  } catch {
    toast.error('读取注册设置失败')
    await router.replace('/login')
  }
})
</script>

<template>
  <AuthCard title="创建账户" description="新账号为普通用户，可以使用默认存储上传。">
    <form class="grid gap-3" novalidate :aria-busy="!ready" @submit.prevent="submit">
      <UiField label="用户名" for="register-username" hint="3–50 个字符" :error="errors.username">
        <UiInput id="register-username" v-model="form.username" maxlength="50" autocomplete="username" autofocus :invalid="!!errors.username" :disabled="busy || !ready" />
      </UiField>
      <UiField label="密码" for="register-password" hint="至少 6 位" :error="errors.password">
        <UiPasswordInput id="register-password" v-model="form.password" maxlength="100" autocomplete="new-password" :invalid="!!errors.password" :disabled="busy || !ready" />
      </UiField>
      <UiField label="确认密码" for="register-confirm" :error="errors.confirm">
        <UiPasswordInput id="register-confirm" v-model="form.confirm" maxlength="100" autocomplete="new-password" :revealable="false" :invalid="!!errors.confirm" :disabled="busy || !ready" />
      </UiField>
      <div v-if="formError" class="flex items-center gap-1.5 rounded-[9px] bg-oi-danger-soft px-2.5 py-2 text-[12.5px] text-oi-danger" role="alert">
        <i class="ri-error-warning-line" aria-hidden="true" />{{ formError }}
      </div>
      <UiButton variant="primary" type="submit" class="mt-1 !h-10" :disabled="busy || !ready">
        <i v-if="busy" class="ri-loader-4-line animate-spin" aria-hidden="true" />
        {{ busy ? '正在注册' : '注册' }}
      </UiButton>
    </form>
    <template #footer>
      已有账号？<RouterLink to="/login" class="font-[550] text-oi-accent-2 hover:underline">返回登录</RouterLink>
    </template>
  </AuthCard>
</template>
