<template>
    <div class="flex min-h-[calc(100vh-140px)] items-center justify-center p-4">
        <!-- 全局加载遮罩 -->
        <div v-if="isLoading" class="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50">
            <div class="loading-card m-[15px] w-full max-w-md rounded-xl bg-white p-6 shadow-2xl dark:bg-gray-800">
                <!-- 加载动画 -->
                <div class="loading-spinner w-12 h-12 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mx-auto mb-4"></div>
                <h3 class="loading-title text-lg font-bold text-center text-gray-800 dark:text-white mb-2">{{ loadingTitle }}</h3>
                <p class="loading-text text-center text-gray-600 dark:text-gray-300 mb-4">{{ loadingText }}</p>
                <!-- 进度条 -->
                <div class="loading-progress h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                    <div class="progress-bar h-full bg-primary dark:bg-primary transition-all duration-300 ease-out" :style="{ width: loadingProgress + '%' }"></div>
                </div>
            </div>
        </div>

        <!-- 登录卡片 -->
        <div class="card glass-panel w-full max-w-md overflow-hidden transition-all duration-300" :class="{ 'opacity-50 pointer-events-none': isLoading }">
            <div class="card-body p-6">
				<div v-if="loginConfig.start_register" class="mb-2 flex justify-end">
					<router-link to="/register" class="text-sm text-primary hover:underline">注册账户</router-link>
				</div>
                <div class="mb-8 text-center">
                    <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-primary to-blue-700 text-2xl text-white shadow-lg shadow-primary/20">
                        <i class="ri-lock-2-line"></i>
                    </div>
                    <h5 class="card-title text-2xl font-bold text-gray-800 dark:text-white">欢迎登录</h5>
                </div>
                <!-- 用户名输入 -->
                <div class="form-group mb-6">
                    <label for="username" class="form-label block text-gray-700 dark:text-gray-300 mb-2">用户名</label>
                    <input 
                        type="text" 
                        v-model="username" 
                        class="input-modern"
                        placeholder="用户名"
                        :disabled="isLoading"
                        @keyup.enter="handleLogin"
                    />
                </div>
                <!-- 密码输入 -->
                <div class="form-group mb-8">
                    <label for="password" class="form-label block text-gray-700 dark:text-gray-300 mb-2">密码</label>
                    <input 
                        type="password" 
                        v-model="password" 
                        class="input-modern"
                        placeholder="密码"
                        :disabled="isLoading"
                        @keyup.enter="handleLogin"
                    />
                </div>
                <!-- 登录按钮 -->
                <div class="form-group">
                    <button 
                        @click="handleLogin" 
                        class="login-btn primary-button w-full py-3 text-base"
                        :class="{ 'opacity-70 cursor-not-allowed': isLoading }"
                        :disabled="isLoading"
                    >
                        登录
                    </button>
                </div>
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
const loadingTitle = ref('');
const loadingText = ref('');
const loadingProgress = ref(0);

// 登录配置
const loginConfig = reactive({
    start_register: false
})

// 加载状态管理
const setLoadingState = (title, text, progress = 0) => {
    isLoading.value = true;
    loadingTitle.value = title;
    loadingText.value = text;
    loadingProgress.value = progress;
};

const clearLoadingState = () => {
    isLoading.value = false;
    loadingTitle.value = '';
    loadingText.value = '';
    loadingProgress.value = 0;
};

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
    setLoadingState('登录中', '正在验证用户信息...', 90);
    
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

            clearLoadingState();
            window.location.replace('/');
        } else {
            clearLoadingState();
            message.error('登录失败: ' + (result.message || '未知错误'));
        }
    } catch (error) {
        clearLoadingState();
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
    // 修复URL方法兼容问题
    if (!URL.revokeObjectUrl && URL.revokeObjectURL) {
        URL.revokeObjectUrl = URL.revokeObjectURL;
    }

    await getLoginSettings();
});
</script>
