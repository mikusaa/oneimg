<template>
  <div class="min-h-[calc(100vh-var(--app-header-height-mobile))] flex items-center justify-center px-4 py-8 md:min-h-[calc(100vh-var(--app-header-height))]">
    <div class="section-card w-full max-w-md p-6 md:p-8">
      <div class="mb-7 flex items-start justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-slate-900 dark:text-white">创建账户</h1>
          <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">注册后使用本地默认存储上传图片</p>
        </div>
        <router-link to="/login" class="text-sm text-primary hover:underline">返回登录</router-link>
      </div>

      <form class="space-y-5" @submit.prevent="handleSubmit">
        <div>
          <label class="field-label" for="register-username">用户名</label>
          <input id="register-username" v-model.trim="form.username" class="input-modern" maxlength="50" autocomplete="username" placeholder="3-50 个字符" :disabled="loading" />
        </div>
        <div>
          <label class="field-label" for="register-password">密码</label>
          <div class="relative">
            <input id="register-password" v-model="form.password" :type="showPassword ? 'text' : 'password'" class="input-modern pr-11" maxlength="100" autocomplete="new-password" placeholder="至少 6 个字符" :disabled="loading" />
            <button type="button" class="pressable absolute right-1.5 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-white/5 dark:hover:text-white" :aria-label="showPassword ? '隐藏密码' : '显示密码'" :title="showPassword ? '隐藏密码' : '显示密码'" @click="showPassword = !showPassword">
              <i :class="showPassword ? 'ri-eye-off-line' : 'ri-eye-line'"></i>
            </button>
          </div>
        </div>
        <div>
          <label class="field-label" for="register-confirm-password">确认密码</label>
          <input id="register-confirm-password" v-model="form.confirmPassword" :type="showPassword ? 'text' : 'password'" class="input-modern" maxlength="100" autocomplete="new-password" placeholder="再次输入密码" :disabled="loading" />
        </div>
        <button type="submit" class="primary-button w-full justify-center py-3" :disabled="loading">
          <i :class="loading ? 'ri-loader-4-line animate-spin' : 'ri-user-add-line'"></i>
          {{ loading ? '注册中' : '注册' }}
        </button>
      </form>
    </div>

  </div>
</template>

<script setup lang="ts">
import { apiFetch } from "@/api/client.ts"
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import message from '@/utils/message.ts'

const router = useRouter()
const form = reactive({ username: '', password: '', confirmPassword: '' })
const config = reactive({ start_register: false })
const loading = ref(false)
const showPassword = ref(false)

const validate = () => {
  if (form.username.length < 3 || form.username.length > 50) return '用户名长度必须在 3-50 个字符之间'
  if (form.password.length < 6 || form.password.length > 100) return '密码长度必须在 6-100 个字符之间'
  if (form.password !== form.confirmPassword) return '两次输入的密码不一致'
  return ''
}

const submitRegistration = async () => {
  loading.value = true
  try {
    const response = await apiFetch('/api/v1/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: form.username, password: form.password })
    })
    const result = await response.json()
    if (!response.ok || !result.data) throw new Error(result.detail || '注册失败')
    message.success('注册成功，请登录')
    await router.replace('/login')
  } catch (error) {
    message.error(error.message || '注册失败')
  } finally {
    loading.value = false
  }
}

const handleSubmit = () => {
  const error = validate()
  if (error) return message.error(error)
  submitRegistration()
}

onMounted(async () => {
  try {
    const response = await apiFetch('/api/v1/public/config')
    const result = await response.json()
    if (!response.ok || !result.data) throw new Error(result.detail || '读取注册设置失败')
    Object.assign(config, { start_register: result.data.registration_enabled })
    if (!config.start_register) {
      message.warning('暂未开放注册')
      return router.replace('/login')
    }
  } catch (error) {
    message.error(error.message || '读取注册设置失败')
    router.replace('/login')
  }
})
</script>
