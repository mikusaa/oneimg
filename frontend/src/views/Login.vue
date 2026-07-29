<template>
    <div class="flex min-h-[calc(100vh-160px)] items-center justify-center px-4 py-10">
        <div class="glass-panel w-full max-w-sm overflow-hidden">
            <div class="p-6 sm:p-7">
				<div v-if="loginConfig.start_register" class="mb-2 flex justify-end">
					<router-link to="/register" class="rounded-md px-1 py-0.5 text-sm font-medium text-primary hover:text-primary-dark">注册账户</router-link>
				</div>
                <div class="mb-7 text-center">
                    <div class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-xl bg-primary text-xl text-white shadow-md shadow-primary/20">
                        <i class="ri-lock-2-line"></i>
                    </div>
                    <h1 class="text-xl font-semibold text-slate-900 dark:text-white">欢迎登录</h1>
                </div>
                <form class="space-y-5" @submit.prevent="handleLogin">
                  <div>
                    <label for="username" class="field-label">用户名</label>
                    <input 
                        id="username"
                        type="text" 
                        v-model="username" 
                        class="input-modern"
                        placeholder="用户名"
                        autocomplete="username"
                        autofocus
                        :disabled="isLoading"
                    />
                  </div>
                  <div>
                    <label for="password" class="field-label">密码</label>
                    <div class="relative">
                    <input 
                        id="password"
                        :type="showPassword ? 'text' : 'password'"
                        v-model="password" 
                        class="input-modern pr-12"
                        placeholder="密码"
                        autocomplete="current-password"
                        :disabled="isLoading"
                    />
                    <button
                      type="button"
                      class="pressable absolute right-1.5 top-1/2 flex h-10 w-10 -translate-y-1/2 items-center justify-center rounded-lg text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-white/5 dark:hover:text-white"
                      :aria-label="showPassword ? '隐藏密码' : '显示密码'"
                      :title="showPassword ? '隐藏密码' : '显示密码'"
                      @click="showPassword = !showPassword"
                    >
                      <i :class="showPassword ? 'ri-eye-off-line' : 'ri-eye-line'"></i>
                    </button>
                    </div>
                  </div>
                    <button 
                        type="submit"
                        class="primary-button w-full py-3 text-base"
                        :disabled="isLoading"
                    >
                        <i v-if="isLoading" class="ri-loader-4-line animate-spin" aria-hidden="true"></i>
                        {{ isLoading ? '正在登录' : '登录' }}
                    </button>
                </form>
            </div>
        </div>

    </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue';
import message from '@/utils/message.js';

// 响应式数据
const username = ref('');
const password = ref('');
const isLoading = ref(false);
const showPassword = ref(false);

// 登录配置
const loginConfig = reactive({
    start_register: false
})

// 加载状态管理
// 登录处理
const handleLogin = () => {
    if (isLoading.value) return;
    
    if (!username.value || !password.value) {
        message.warning('请输入用户名和密码');
        return;
    }
    
    putLogin();
};

// 提交登录请求
const putLogin = async () => {
    isLoading.value = true;
    
    try {
        // 组装登录参数
        const loginData = {
            username: username.value,
            password: password.value
        };

        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(loginData)
        });
        
        const result = await response.json();
        
        if (response.ok && result.code === 200) {
            // 保存用户信息
            const userInfo = {
                id: result.data?.user?.id,
				username: username.value,
                role: result.data?.user?.role,
				permission: result.data?.user?.permission || { codes: [], buckets: [] }
            };
            localStorage.setItem('userInfo', JSON.stringify(userInfo));

            window.location.replace('/');
        } else {
            isLoading.value = false;
            message.error('登录失败: ' + (result.message || '未知错误'));
        }
    } catch (error) {
        isLoading.value = false;
        message.error('登录请求失败，请检查网络连接: ' + error.message);
    }
};

const getLoginSettings = async () => { 
    try {
        const response = await fetch('/api/settings/login', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        const result = await response.json();
        if (response.ok && result.code === 200) {
            Object.assign(loginConfig, result.data);
        } else {
            message.error('获取登录配置失败');
        }
    } catch (error) {
        message.error('获取登录配置失败: ' + error.message);
    }
};

onMounted(async () => {
    await getLoginSettings();
});
</script>
