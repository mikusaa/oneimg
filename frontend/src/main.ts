import { createApp } from 'vue'
import App from './App.vue'
import router from './router/router.ts'
import './assets/main.css'

// 主题管理器已经在导入时自动初始化
const app = createApp(App)

app.use(router)
app.mount('#app')
