import { AppRouteRecordRaw } from '@/utils/router'

/**
 * 静态路由配置（不需要权限就能访问的路由）
 *
 * 属性说明：
 * isHideTab: true 表示不在标签页中显示
 *
 * 注意事项：
 * 1、path、name 不要和动态路由冲突，否则会导致路由冲突无法访问
 * 2、静态路由不管是否登录都可以访问
 */
export const staticRoutes: AppRouteRecordRaw[] = [
  {
    path: '/',
    name: 'RootRedirect',
    redirect: '/user/login'
  },
  {
    path: '/install',
    name: 'Install',
    component: () => import('@views/install/index.vue'),
    meta: { title: '系统安装', isHideTab: true }
  },
  // 不需要登录就能访问的路由示例
  // {
  //   path: '/welcome',
  //   name: 'WelcomeStatic',
  //   component: () => import('@views/dashboard/console/index.vue'),
  //   meta: { title: 'menus.dashboard.title' }
  // },
  {
    path: '/admin',
    name: 'Login',
    component: () => import('@views/auth/login/index.vue'),
    meta: { title: 'menus.login.title', isHideTab: true }
  },
  {
    path: '/auth/forget-password',
    name: 'ForgetPassword',
    component: () => import('@views/auth/forget-password/index.vue'),
    meta: { title: 'menus.forgetPassword.title', isHideTab: true }
  },
  {
    path: '/agent-panel/login',
    name: 'AgentLogin',
    component: () => import('@views/agent-panel/login/index.vue'),
    meta: { title: '代理商登录', isHideTab: true }
  },
  {
    path: '/agent',
    redirect: '/agent-panel/login'
  },
  {
    path: '/user/login',
    name: 'UserLogin',
    component: () => import('@views/user-panel/login/index.vue'),
    meta: { title: '授权服务首页', isHideTab: true }
  },
  {
    path: '/user/reset-password',
    name: 'UserResetPassword',
    component: () => import('@views/user-panel/reset-password/index.vue'),
    meta: { title: '重置密码', isHideTab: true }
  },
  // 支付网关回跳地址兼容（后端及历史订单 return_url 指向 /agent/*，重定向到现行代理商面板并保留 query）
  {
    path: '/agent/finance',
    redirect: (to) => ({ path: '/agent-panel/finance', query: to.query })
  },
  {
    path: '/agent/purchase',
    redirect: (to) => ({ path: '/agent-panel/purchase', query: to.query })
  },
  {
    path: '/agent-panel',
    name: 'AgentPanel',
    redirect: '/agent-panel/login',
    component: () => import('@views/agent-panel/layout/index.vue'),
    meta: { title: '代理商面板', isHideTab: true },
    children: [
      {
        path: 'dashboard',
        name: 'AgentPanelDashboard',
        component: () => import('@views/agent-panel/dashboard/index.vue'),
        meta: { title: '概览' }
      },
      {
        path: 'licenses',
        name: 'AgentPanelLicenses',
        component: () => import('@views/agent-panel/licenses/index.vue'),
        meta: { title: '我的授权' }
      },
      {
        path: 'finance',
        name: 'AgentPanelFinance',
        component: () => import('@views/agent-panel/finance/index.vue'),
        meta: { title: '我的财务' }
      },
      {
        path: 'purchase',
        name: 'AgentPanelPurchase',
        component: () => import('@views/agent-panel/purchase/index.vue'),
        meta: { title: '开通授权' }
      },
      {
        path: 'profile',
        name: 'AgentPanelProfile',
        component: () => import('@views/agent-panel/profile/index.vue'),
        meta: { title: '个人设置' }
      }
    ]
  },
  {
    path: '/user',
    name: 'UserPanel',
    redirect: '/user/login',
    component: () => import('@views/user-panel/layout/index.vue'),
    meta: { title: '用户面板', isHideTab: true },
    children: [
      {
        path: 'dashboard',
        name: 'UserPanelDashboard',
        component: () => import('@views/user-panel/dashboard/index.vue'),
        meta: { title: '概览' }
      },
      {
        path: 'licenses',
        name: 'UserPanelLicenses',
        component: () => import('@views/user-panel/licenses/index.vue'),
        meta: { title: '我的授权' }
      },
      {
        path: 'purchase',
        name: 'UserPanelPurchase',
        component: () => import('@views/user-panel/purchase/index.vue'),
        beforeEnter: (to) => {
          const order = typeof to.query.rechargeOrder === 'string' ? to.query.rechargeOrder : ''
          if (order.startsWith('LP')) {
            return { path: '/agent-panel/purchase', query: to.query }
          }
          return true
        },
        meta: { title: '购买授权' }
      },
      {
        path: 'profile',
        name: 'UserPanelProfile',
        component: () => import('@views/user-panel/profile/index.vue'),
        meta: { title: '个人设置' }
      }
    ]
  },
  {
    path: '/403',
    name: 'Exception403',
    component: () => import('@views/exception/403/index.vue'),
    meta: { title: '403', isHideTab: true }
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'Exception404',
    component: () => import('@views/exception/404/index.vue'),
    meta: { title: '404', isHideTab: true }
  },
  {
    path: '/500',
    name: 'Exception500',
    component: () => import('@views/exception/500/index.vue'),
    meta: { title: '500', isHideTab: true }
  },
  {
    path: '/outside',
    component: () => import('@views/index/index.vue'),
    name: 'Outside',
    meta: { title: 'menus.outside.title' },
    children: [
      // iframe 内嵌页面
      {
        path: '/outside/iframe/:path',
        name: 'Iframe',
        component: () => import('@/views/outside/Iframe.vue'),
        meta: { title: 'iframe' }
      }
    ]
  }
]
