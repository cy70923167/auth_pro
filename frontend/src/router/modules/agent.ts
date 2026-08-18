import { AppRouteRecord } from '@/types/router'

export const agentRoutes: AppRouteRecord = {
  path: '/admin/agent',
  name: 'Agent',
  component: '/index/index',
  meta: {
    title: '代理商管理',
    icon: 'ri:team-line',
    roles: ['R_SUPER', 'R_ADMIN']
  },
  children: [
    {
      path: 'list',
      name: 'AgentList',
      component: '/agent/list',
      meta: {
        title: '代理商列表',
        icon: 'ri:user-star-line',
        keepAlive: true
      }
    },
    {
      path: 'level',
      name: 'AgentLevel',
      component: '/agent/level',
      meta: {
        title: '等级管理',
        icon: 'ri:vip-crown-line',
        keepAlive: true
      }
    },
    {
      path: 'recharge',
      name: 'AgentRecharge',
      component: '/agent/recharge',
      meta: {
        title: '财务流水',
        icon: 'ri:money-cny-circle-line',
        keepAlive: true
      }
    },
    {
      path: 'quota',
      name: 'AgentQuota',
      component: '/agent/quota',
      meta: {
        title: '开码配额',
        icon: 'ri:key-2-line',
        keepAlive: true
      }
    }
  ]
}