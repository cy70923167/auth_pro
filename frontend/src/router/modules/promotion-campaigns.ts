import { AppRouteRecord } from '@/types/router'

export const promotionCampaignRoutes: AppRouteRecord = {
  path: '/promotion-campaigns',
  name: 'PromotionCampaigns',
  component: '/promotion-campaigns/index',
  meta: {
    title: 'menus.promotionCampaigns',
    icon: 'ri:discount-percent-line',
    keepAlive: true,
    roles: ['R_SUPER', 'R_ADMIN']
  }
}