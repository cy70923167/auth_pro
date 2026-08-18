import request from '@/utils/http'

export type PromotionAudience = 'user' | 'agent' | 'all'
export type PromotionRuleType = 'discount' | 'reduction' | 'fixed_price'
export type PromotionCampaignStatus = 'active' | 'upcoming' | 'ended' | 'disabled'

export interface PromotionAppOption {
  id: number
  name: string
}

export interface PromotionPlanOption {
  id: number
  appId: number
  appName: string
  name: string
  durationDays: number
  durationText: string
  price: number
  enabled: boolean
}

export interface PromotionCampaignPlan {
  planId: number
  planName: string
  originalPrice: number
  ruleType: PromotionRuleType
  value: number
}

export interface PromotionCampaignItem {
  id: number
  appId: number
  appName: string
  name: string
  audience: PromotionAudience
  startsAt: string
  endsAt: string
  enabled: boolean
  status: PromotionCampaignStatus
  createdAt: string
  updatedAt: string
  plans: PromotionCampaignPlan[]
}

export interface PromotionCampaignPayload {
  appId: number
  name: string
  audience: PromotionAudience
  startsAt: string
  endsAt: string
  enabled: boolean
  plans: Array<{
    planId: number
    ruleType: PromotionRuleType
    value: number
  }>
}

export interface PromotionCampaignQuery {
  appId?: number
  keyword?: string
  audience?: PromotionAudience
  status?: PromotionCampaignStatus
}

export function fetchPromotionCampaigns(params: PromotionCampaignQuery = {}) {
  return request.get<PromotionCampaignItem[]>({
    url: '/api/promotion/campaigns',
    params
  })
}

export function createPromotionCampaign(data: PromotionCampaignPayload) {
  return request.post<{ id: number }>({
    url: '/api/promotion/campaigns',
    data
  })
}

export function updatePromotionCampaign(id: number, data: PromotionCampaignPayload) {
  return request.put<void>({
    url: `/api/promotion/campaigns/${id}`,
    data
  })
}

export function togglePromotionCampaign(id: number, enabled: boolean) {
  return request.put<void>({
    url: `/api/promotion/campaigns/${id}/toggle`,
    data: { enabled }
  })
}

export function deletePromotionCampaign(id: number) {
  return request.del<void>({
    url: `/api/promotion/campaigns/${id}`
  })
}

export function fetchPromotionApps() {
  return request.get<PromotionAppOption[]>({ url: '/api/license/apps' })
}

export function fetchPromotionPlans(appId: number) {
  return request.get<PromotionPlanOption[]>({
    url: '/api/plan/list',
    params: { appId }
  })
}