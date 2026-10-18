import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { apiFetch } from '@/api/client.ts'
import { toast } from '@/components/ui/toast'
import { clearSession } from '@/composables/useSession'

export const useLogout = () => {
  const router = useRouter()
  const pending = ref(false)

  const logout = async () => {
    if (pending.value) return
    pending.value = true
    try {
      const response = await apiFetch('/api/v1/auth/logout', { method: 'POST' })
      // 401 说明会话已经失效，同样按已退出处理
      if (!response.ok && response.status !== 401) throw new Error(`退出登录失败：${response.status}`)
      clearSession()
      toast.success('已退出登录')
      await router.push('/login')
    } catch {
      toast.error('退出登录失败，请稍后重试')
    } finally {
      pending.value = false
    }
  }

  return { logout, pending }
}
