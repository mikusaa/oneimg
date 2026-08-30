import { apiFetch } from "@/api/client.ts"
import { createRouter, createWebHistory, type RouteLocationNormalized, type RouteRecordRaw } from 'vue-router'
import { hasAnyPermission, ROLE_ADMIN } from '@/utils/permissions.ts'
import Message from '@/utils/message.ts'

type SeoSettings = Record<'seo_title' | 'seo_description' | 'seo_keywords' | 'seo_icp' | 'public_security' | 'seo_icon', string>
interface StoredUser {
  username?: string
  role?: number
  permission?: { codes?: string[]; bucket_ids?: number[] }
}

let seoStting = {
  seo_title: '初春图床',
  seo_description: '',
  seo_keywords: '',
  seo_icp: '',
  public_security: '',
  seo_icon: ''
};

const seoBus = {
  callbacks: [],
  onUpdate: (cb) => seoBus.callbacks.push(cb),
  triggerUpdate: (data) => seoBus.callbacks.forEach(cb => cb(data))
};

let seoRequestPromise = null;

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { 
      title: '登录', 
      public: true 
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register.vue'),
    meta: { title: '注册', public: true }
  },
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/Home.vue'),
    meta: {
      title: '首页'
    }
  },
  {
    path: '/gallery',
    name: 'Gallery',
    component: () => import('@/views/Gallery.vue'),
    meta: {
      title: '图库'
    }
  },
  {
    path: '/tags',
    redirect: to => ({ path: '/gallery', query: { ...to.query, view: 'tags' } })
  },
  {
    path: '/stats',
    redirect: '/'
  },
  {
    path: '/buckets',
    name: 'Buckets',
    component: () => import('@/views/Buckets.vue'),
    meta: {
      title: '存储列表',
      permissions: ['storage:create', 'storage:update', 'storage:delete']
    }
  },
  {
    path: '/users',
    name: 'Users',
    component: () => import('@/views/Users.vue'),
    meta: {
      title: '用户管理',
      permissions: ['user:list']
    }
  },
  {
    path: '/account',
    name: 'Account',
    component: () => import('@/views/Account.vue'),
    meta: { 
      title: '账户设置'
    }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/Settings.vue'),
    meta: { 
      title: '系统设置',
      permissions: ['setting:upload', 'setting:image', 'setting:security', 'setting:seo']
    }
  }
]

// 创建路由实例
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes
})

// 获取SEO配置
const getSeoSetting = async () => {
  if (seoRequestPromise) return seoRequestPromise;

  seoRequestPromise = new Promise(async (resolve) => {
    try {
      const response = await apiFetch('/api/v1/public/config', {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' }
      });

      if (!response.ok) {
        throw new Error(`请求失败：${response.status} ${response.statusText}`);
      }

      const result = await response.json();

      if (result.data) {
        const site = result.data.site || {};
        seoStting = {
          ...seoStting,
          seo_title: site.title || seoStting.seo_title,
          seo_description: site.description || seoStting.seo_description,
          seo_keywords: site.keywords || seoStting.seo_keywords,
          seo_icp: site.icp || seoStting.seo_icp,
          public_security: site.public_security || seoStting.public_security,
          seo_icon: site.icon || seoStting.seo_icon
        };
        window.seoStting = seoStting; // 挂载到全局
        seoBus.triggerUpdate(seoStting); // 更新SEO设置

        // 设置网站图标
        if (seoStting.seo_icon) {
          let favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
          if (!favicon) {
            favicon = document.createElement('link');
            favicon.rel = 'icon';
            favicon.type = 'image/x-icon';
            document.head.appendChild(favicon);
          }
          favicon.href = seoStting.seo_icon;
        }
      } else {
        Message.error(result.detail || '获取SEO设置失败：无数据');
      }
    } catch (error) {
      console.error('获取SEO设置失败:', error);
      Message.error(error instanceof Error ? error.message : '获取SEO设置失败：网络异常');
    } finally {
      resolve(seoStting);
    }
  });

  return seoRequestPromise;
};

// 封装动态标题计算函数
const getPageTitle = (to: RouteLocationNormalized) => {
  if (to.meta.title === '首页') {
    return seoStting.seo_title;
  }
  return to.meta.title ? `${to.meta.title} - ${seoStting.seo_title}` : seoStting.seo_title;
};

//全局前置守卫
router.beforeEach(async (to, from, next) => {
  try {
    // 等待SEO接口完成
    await getSeoSetting();

    // 设置页面标题
    document.title = getPageTitle(to);

    // 处理公开路由
    const isPublic = to.meta.public;
    if (isPublic) {
      return next();
    }

    // 验证本地用户信息
    let userInfo: StoredUser = {};
    try {
      userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
    } catch (error) {
      localStorage.removeItem('userInfo');
      userInfo = {};
    }
    if (!userInfo.username) {
      window.refreshNavItems && window.refreshNavItems();
      return next('/login');
    }

    // 验证登录状态
    const response = await apiFetch('/api/v1/me');
    if (!response.ok) {
      // 删除本地用户信息
      localStorage.removeItem('userInfo');
      window.refreshNavItems && window.refreshNavItems();
      throw new Error(`登录状态验证失败：${response.status}`);
    }

    const result = await response.json();
    if (!result.data || !result.data.username) {
      localStorage.removeItem('userInfo');
      window.refreshNavItems && window.refreshNavItems();
      return next('/login');
    }

    // 验证用户名一致性
    if (userInfo.username !== result.data.username) {
      localStorage.removeItem('userInfo');
      window.refreshNavItems && window.refreshNavItems();
      return next('/login');
    }
    const currentRole = result.data.role;
    userInfo.role = currentRole;
    userInfo.permission = result.data.permission || { codes: [], bucket_ids: [] };
    localStorage.setItem('userInfo', JSON.stringify(userInfo));
    window.refreshNavItems && window.refreshNavItems();
    if (Array.isArray(to.meta.permissions)) {
      if (Number(currentRole) !== ROLE_ADMIN || !hasAnyPermission(to.meta.permissions, userInfo)) return next('/');
    }

    // 所有验证通过，放行
    next();
  } catch (error) {
    // 避免多次调用next的警告
    if (!to.fullPath.includes('/login')) {
      next('/login');
    } else {
      next();
    }
  }
});

// 全局后置守卫
router.afterEach((to) => {
  document.title = getPageTitle(to);
});

window.seoBus = seoBus;

export default router
