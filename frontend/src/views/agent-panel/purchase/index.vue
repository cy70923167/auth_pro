<template>
  <div class="agent-purchase">
    <div class="steps-bar">
      <template v-for="(item, index) in stepItems" :key="item.value">
        <div
          class="step"
          :class="{ active: step >= item.value, done: step > item.value }"
          @click="goStep(item.value)"
        >
          <div class="step-dot">{{ item.value }}</div>
          <span class="step-text">{{ item.label }}</span>
        </div>
        <div
          v-if="index < stepItems.length - 1"
          class="step-line"
          :class="{ active: step > item.value }"
        ></div>
      </template>
    </div>

    <transition name="fade" mode="out-in">
      <div v-if="step === 1" key="step1" class="step-content">
        <div v-if="appList.length === 0" class="empty-card">
          暂无可开通应用，请联系管理员先启用应用和套餐
        </div>
        <div v-else class="app-grid">
          <div
            v-for="app in appList"
            :key="app.id"
            class="app-card"
            :class="{ active: formData.appId === app.id, 'has-promo': hasPromotion(app) }"
            @click="selectApp(app.id)"
          >
            <div v-if="hasPromotion(app)" class="app-promo-badge">
              <iconify-icon icon="ri:flashlight-fill" width="14" />
              <span>限时活动</span>
            </div>
            <div class="app-icon-wrap">
              <iconify-icon :icon="app.icon" width="26" />
            </div>
            <h4 class="app-name">{{ app.name }}</h4>
            <p class="app-desc">{{ app.desc }}</p>
            <div class="app-footer">
              <div class="app-pricing">
                <span class="price-currency">¥</span>
                <span class="price-amount">{{ minPlanPrice(app).toFixed(2) }}</span>
                <span class="price-unit">起</span>
              </div>
              <div class="plan-count">{{ app.plans.length }} 个套餐</div>
            </div>
            <div class="app-check-icon" v-if="formData.appId === app.id">
              <iconify-icon icon="ri:checkbox-circle-fill" width="22" />
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="step === 2" key="step2" class="step-content">
        <div class="config-layout">
          <div class="config-main">
            <div class="config-section">
              <label class="config-label">授权类型</label>
              <div class="type-options">
                <div
                  v-for="t in availableTypeOptions"
                  :key="t.value"
                  class="type-chip"
                  :class="{ active: formData.type === t.value }"
                  @click="selectType(t.value)"
                >
                  <iconify-icon :icon="t.icon" width="18" />
                  <span>{{ t.label }}</span>
                </div>
              </div>
            </div>

            <div class="config-section">
              <label class="config-label">授权归属</label>
              <el-select
                v-model="formData.userId"
                class="owner-select"
                filterable
                remote
                reserve-keyword
                clearable
                :remote-method="fetchUserOptions"
                :loading="userLoading"
                placeholder="留空归属当前代理，输入用户邮箱搜索"
                @visible-change="handleUserSelectVisible"
              >
                <el-option
                  v-for="user in userOptions"
                  :key="user.id"
                  :label="formatUserOption(user)"
                  :value="user.id"
                />
              </el-select>
              <span class="config-hint">选择用户后，授权将直接归属该用户。</span>
            </div>

            <div class="config-section">
              <label class="config-label">{{ domainLabel }}</label>
              <div class="domain-input-wrap">
                <iconify-icon :icon="domainIcon" width="18" class="input-icon" />
                <input
                  v-model="formData.domain"
                  :disabled="formData.type === 'key'"
                  :placeholder="domainPlaceholder"
                  class="domain-input"
                />
              </div>
            </div>

            <div class="config-section">
              <label class="config-label">选择套餐</label>
              <div class="plan-grid">
                <div
                  v-for="plan in filteredAppPlans"
                  :key="plan.id"
                  class="plan-card"
                  :class="{ active: formData.planId === plan.id, 'has-promo': plan.promotion }"
                  @click="selectPlan(plan.id)"
                >
                  <div v-if="plan.promotion" class="promo-badge">{{ plan.promotion.name }}</div>
                  <div class="plan-head">
                    <span class="plan-name">{{ plan.name }}</span>
                    <span v-if="plan.promotion" class="plan-tag promo">{{
                      promoRuleText(plan.promotion)
                    }}</span>
                    <span v-else-if="planHasDiscount(plan)" class="plan-tag">{{
                      formatDiscount(plan.discount)
                    }}</span>
                  </div>
                  <div class="plan-pricing">
                    <span class="plan-currency">¥</span>
                    <span class="plan-amount">{{ Number(plan.price).toFixed(2) }}</span>
                    <span v-if="planHasDiscount(plan)" class="plan-original"
                      >¥{{ Number(plan.originalPrice).toFixed(2) }}</span
                    >
                  </div>
                  <div class="plan-meta">
                    <span class="plan-duration">{{ plan.durationText }}</span>
                    <span v-if="plan.promotion" class="plan-time"
                      >活动截止：{{ plan.promotion.endsAt }}</span
                    >
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="config-preview">
            <div class="preview-card">
              <div class="preview-header">
                <iconify-icon :icon="selectedApp?.icon || 'ri:apps-line'" width="24" />
                <span>{{ selectedApp?.name }}</span>
              </div>
              <div class="preview-body">
                <div class="preview-item">
                  <span class="preview-key">套餐</span>
                  <span class="preview-val">{{ selectedPlan?.name }}</span>
                </div>
                <div class="preview-item">
                  <span class="preview-key">时长</span>
                  <span class="preview-val">{{ selectedPlan?.durationText }}</span>
                </div>
                <div class="preview-item">
                  <span class="preview-key">类型</span>
                  <span class="preview-val">{{ typeLabels[formData.type] }}</span>
                </div>
                <div class="preview-item">
                  <span class="preview-key">目标</span>
                  <span class="preview-val mono">{{ displayTarget }}</span>
                </div>
              </div>
              <div class="preview-footer">
                <span class="preview-total-label">折后价格</span>
                <div class="preview-price-wrap">
                  <span v-if="hasDiscount" class="preview-original-price"
                    >¥{{ originalPrice.toFixed(2) }}</span
                  >
                  <span class="preview-total-price">¥{{ computedCost.toFixed(2) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="step === 3" key="step3" class="step-content">
        <div class="payment-layout">
          <div class="payment-detail-card">
            <div class="payment-header">
              <iconify-icon icon="ri:file-list-3-line" width="20" />
              <span>订单详情</span>
            </div>
            <div class="payment-rows">
              <div class="payment-row">
                <span>应用</span><span>{{ selectedApp?.name }}</span>
              </div>
              <div class="payment-row">
                <span>套餐</span><span>{{ selectedPlan?.name }}</span>
              </div>
              <div class="payment-row">
                <span>授权时长</span><span>{{ selectedPlan?.durationText }}</span>
              </div>
              <div class="payment-row">
                <span>归属</span><span class="mono">{{ ownerLabel }}</span>
              </div>
              <div class="payment-row">
                <span>授权类型</span><span>{{ typeLabels[formData.type] }}</span>
              </div>
              <div class="payment-row">
                <span>授权目标</span><span class="mono">{{ displayTarget }}</span>
              </div>
              <div class="payment-row">
                <span>套餐原价</span><span>¥{{ originalPrice.toFixed(2) }}</span>
              </div>
              <div v-if="!usePromotionPath" class="payment-row discount-row">
                <span>代理折扣</span>
                <span
                  >{{ formatDiscount(agentDiscount)
                  }}<template v-if="hasAgentDiscount"
                    >，优惠 ¥{{ agentDiscountAmount.toFixed(2) }}</template
                  ></span
                >
              </div>
              <div v-if="usePromotionPath" class="payment-row discount-row">
                <span
                  >活动优惠{{
                    selectedPromotion?.name ? `（${selectedPromotion.name}）` : ''
                  }}</span
                >
                <span
                  >{{ promoRuleText(selectedPromotion) }}，优惠 ¥{{
                    promotionSavings.toFixed(2)
                  }}</span
                >
              </div>
            </div>
            <div class="payment-total-row">
              <span>应付金额</span>
              <div class="payment-price">
                <span class="price-sym">¥</span>
                <span class="price-num">{{ computedCost.toFixed(2) }}</span>
              </div>
            </div>
          </div>

          <div class="payment-action-card">
            <div class="payment-method">
              <label class="config-label">支付方式</label>
              <div class="method-options">
                <div
                  v-for="option in payOptions"
                  :key="option.code"
                  class="method-item"
                  :class="{ active: payMethod === option.code }"
                  @click="payMethod = option.code"
                >
                  <iconify-icon :icon="option.icon" width="24" :color="option.color" />
                  <span>{{ option.label }}</span>
                  <span v-if="option.code === 'balance'" class="method-balance"
                    >¥{{ agentBalance.toFixed(2) }}</span
                  >
                  <span v-else-if="option.code === 'quota'" class="method-balance"
                    >剩余 {{ quotaInfo.remain }}</span
                  >
                </div>
              </div>
            </div>
            <div
              class="balance-warning"
              v-if="payMethod === 'balance' && agentBalance < computedCost"
            >
              当前余额不足，无法完成购买
            </div>
            <button
              class="purchase-btn"
              :disabled="
                purchasing ||
                (payMethod === 'balance' && agentBalance < computedCost) ||
                (isOnlinePay && computedCost <= 0)
              "
              @click="handlePurchase"
            >
              <iconify-icon icon="ri:secure-payment-line" width="20" />
              <span>{{ purchaseButtonText }}</span>
            </button>
            <p class="payment-hint">
              <iconify-icon icon="ri:shield-check-line" width="14" />
              <template v-if="payMethod === 'quota'"
                >配额支付将消耗 1 个当前应用配额，授权即时生效</template
              >
              <template v-else>套餐价格以后端为准，支付成功后授权即时生效</template>
            </p>
          </div>
        </div>
      </div>

      <div v-else-if="step === 4" key="step4" class="step-content">
        <div class="success-card">
          <div class="success-banner">
            <div class="success-icon">
              <iconify-icon icon="ri:checkbox-circle-fill" width="30" />
            </div>
            <h3>授权已生成</h3>
            <p class="success-sub">授权已即时生效，可在「我的授权」中查看管理</p>
          </div>

          <div class="success-license-no">
            <span class="license-label">授权编号</span>
            <span class="mono license-value">{{ purchaseResult?.licenseNo }}</span>
            <button class="copy-btn" title="复制授权编号" @click="copyLicenseNo">
              <iconify-icon icon="ri:file-copy-line" width="15" />
            </button>
          </div>

          <div class="success-grid">
            <div class="success-item">
              <span class="item-label">应用</span>
              <span class="item-value">{{ purchaseResult?.appName }}</span>
            </div>
            <div class="success-item">
              <span class="item-label">套餐</span>
              <span class="item-value">{{ purchaseResult?.planName }}</span>
            </div>
            <div class="success-item">
              <span class="item-label">授权时长</span>
              <span class="item-value">{{ formatDuration(purchaseResult?.durationDays) }}</span>
            </div>
            <div class="success-item">
              <span class="item-label">支付方式</span>
              <span class="item-value">{{
                payMethodLabels[purchaseResult?.payMethod] || '余额支付'
              }}</span>
            </div>
          </div>

          <div class="success-amount" v-if="purchaseResult?.payMethod !== 'quota'">
            <div class="amount-cell">
              <span class="amount-label">扣款金额</span>
              <span class="amount-value" :class="{ free: Number(purchaseResult?.cost || 0) <= 0 }">
                {{
                  Number(purchaseResult?.cost || 0) <= 0
                    ? '免费'
                    : `¥${Number(purchaseResult?.cost || 0).toFixed(2)}`
                }}
              </span>
            </div>
            <div class="amount-divider"></div>
            <div class="amount-cell">
              <span class="amount-label">剩余余额</span>
              <span class="amount-value">¥{{ agentBalance.toFixed(2) }}</span>
            </div>
          </div>

          <el-button type="primary" class="success-btn" @click="resetFlow">
            <iconify-icon icon="ri:add-line" width="16" style="margin-right: 4px" />
            继续开通
          </el-button>
        </div>
      </div>
    </transition>

    <div class="bottom-nav" v-if="step < 4">
      <el-button v-if="step > 1" @click="step--">上一步</el-button>
      <el-button v-if="step < 3" type="primary" :disabled="!canNext" @click="handleNext">
        下一步
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
  import { ref, reactive, computed, onMounted } from 'vue'
  import { ElMessage } from 'element-plus'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import axios from 'axios'
  import { useRoute, useRouter } from 'vue-router'

  const route = useRoute()
  const router = useRouter()

  const step = ref(1)
  const payMethod = ref('balance')
  const payOptions = ref<any[]>([
    { code: 'balance', label: '余额支付', icon: 'ri:wallet-3-line', color: '#2e7d32' }
  ])
  const quotaInfo = ref({ total: 0, used: 0, remain: 0 })
  const agentBalance = ref(0)
  const purchasing = ref(false)
  const purchaseResult = ref<any>(null)

  const payMethodLabels: Record<string, string> = {
    balance: '余额支付',
    quota: '配额支付',
    alipay: '支付宝',
    wxpay: '微信',
    qqpay: 'QQ支付',
    bank: '网银支付'
  }

  const stepItems = [
    { value: 1, label: '选择应用' },
    { value: 2, label: '授权配置' },
    { value: 3, label: '支付开通' },
    { value: 4, label: '生成授权' }
  ]

  function getToken() {
    return localStorage.getItem('agent_panel_token') || ''
  }

  const authHeaders = computed(() => ({ Authorization: `Bearer ${getToken()}` }))
  const appList = ref<any[]>([])
  type UserOption = { id: number; name: string; email: string }
  const userOptions = ref<UserOption[]>([])
  const userLoading = ref(false)
  let userSearchTimer: ReturnType<typeof setTimeout> | null = null
  let userSearchSequence = 0

  function formatUserOption(user: UserOption) {
    return user.name === user.email ? user.name : `${user.name} (${user.email})`
  }

  function fetchUserOptions(keyword = '') {
    if (userSearchTimer) clearTimeout(userSearchTimer)
    const sequence = ++userSearchSequence
    userSearchTimer = setTimeout(async () => {
      userLoading.value = true
      try {
        const { data } = await axios.get('/api/agent-panel/users/options', {
          headers: authHeaders.value,
          params: { keyword: keyword.trim(), limit: 20 }
        })
        if (sequence === userSearchSequence) {
          userOptions.value = data.code === 200 && Array.isArray(data.data) ? data.data : []
        }
      } catch {
        if (sequence === userSearchSequence) userOptions.value = []
      } finally {
        if (sequence === userSearchSequence) userLoading.value = false
      }
    }, 250)
  }

  function handleUserSelectVisible(visible: boolean) {
    if (visible && userOptions.value.length === 0) fetchUserOptions()
  }

  async function fetchApps() {
    try {
      const { data } = await axios.get('/api/agent-panel/apps/purchase', {
        headers: authHeaders.value
      })
      if (data.code === 200) {
        const apps = (data.data || []).map((a: any) => ({
          ...a,
          icon: a.icon || 'ri:apps-line',
          desc: a.desc || a.name,
          purchaseLicenseTypes: Array.isArray(a.purchaseLicenseTypes)
            ? a.purchaseLicenseTypes
            : typeOptionCatalog.map((option) => option.value),
          plans: (a.plans || []).map((p: any) => ({
            ...p,
            price: normalizePrice(p),
            basePrice: normalizeBasePrice(p),
            originalPrice: normalizeOriginalPrice(p),
            discount: normalizeDiscount(p)
          }))
        }))
        appList.value = apps
        if (formData.appId) {
          const selected = apps.find((app: any) => app.id == formData.appId)
          if (!selected) {
            resetPurchaseSelection()
            step.value = 1
          } else if (!selected.purchaseLicenseTypes.includes(formData.type)) {
            formData.type = selected.purchaseLicenseTypes[0] || ''
            formData.domain = ''
          }
        }
      } else if (data.code === 403) {
        ElMessage.error('请先登录代理端')
      }
    } catch {
      ElMessage.error('加载可开通应用失败')
    }
  }

  function normalizePrice(plan: any) {
    return Number(plan.price ?? plan.Price ?? plan.amount ?? plan.Amount ?? 0)
  }

  function normalizeOriginalPrice(plan: any) {
    return Number(plan.originalPrice ?? plan.OriginalPrice ?? normalizePrice(plan))
  }

  function normalizeBasePrice(plan: any) {
    const base = Number(plan.basePrice ?? plan.BasePrice ?? NaN)
    if (Number.isFinite(base)) return base
    return Math.max(0, (normalizeOriginalPrice(plan) * normalizeDiscount(plan)) / 10)
  }

  function normalizeDiscount(plan: any) {
    return Number(plan.discount ?? plan.Discount ?? 10)
  }

  async function fetchPayOptions() {
    if (!formData.appId) return
    try {
      const { data } = await axios.get('/api/agent-panel/purchase/pay-options', {
        headers: authHeaders.value,
        params: { appId: Number(formData.appId) }
      })
      if (data.code === 200) {
        const options =
          Array.isArray(data.data?.options) && data.data.options.length > 0
            ? data.data.options
            : [{ code: 'balance', label: '余额支付', icon: 'ri:wallet-3-line', color: '#2e7d32' }]
        payOptions.value = options
        quotaInfo.value = data.data?.quota || { total: 0, used: 0, remain: 0 }
        const usable =
          computedCost.value <= 0
            ? options.filter((o: any) => o.code === 'balance' || o.code === 'quota')
            : options
        if (!usable.some((o: any) => o.code === payMethod.value)) {
          payMethod.value = usable[0]?.code || 'balance'
        }
      }
    } catch {
      ElMessage.error('加载支付方式失败')
    }
  }

  async function fetchBalance() {
    try {
      const { data } = await axios.get('/api/agent-panel/balance', { headers: authHeaders.value })
      if (data.code === 200) {
        agentBalance.value = Number(data.data.balance || 0)
      }
    } catch {
      ElMessage.error('加载余额失败')
    }
  }

  // 线上支付回跳：轮询购买订单状态，支付成功后展示授权结果
  let purchasePollTimer: ReturnType<typeof setInterval> | null = null

  function stopPurchasePoll() {
    if (purchasePollTimer) {
      clearInterval(purchasePollTimer)
      purchasePollTimer = null
    }
  }

  async function checkPurchaseReturn() {
    const orderNo = typeof route.query.rechargeOrder === 'string' ? route.query.rechargeOrder : ''
    if (!orderNo || !orderNo.startsWith('LP')) return

    const nextQuery = { ...route.query }
    delete nextQuery.rechargeOrder
    delete nextQuery.rechargeReturn
    router.replace({ query: nextQuery })

    ElMessage.info('正在确认支付结果…')
    let attempts = 0
    purchasePollTimer = setInterval(async () => {
      attempts++
      try {
        const { data } = await axios.get(`/api/agent-panel/purchase/orders/${orderNo}`, {
          headers: authHeaders.value
        })
        const status = data.data?.status
        if (status === 'paid') {
          stopPurchasePoll()
          purchaseResult.value = {
            licenseNo: data.data.licenseNo,
            licenseId: data.data.licenseId,
            orderNo,
            payMethod: data.data.payMethod || 'alipay',
            appName: data.data.appName,
            planName: data.data.planName,
            durationDays: data.data.durationDays,
            cost: Number(data.data.cost || 0)
          }
          agentBalance.value = Number(data.data.newBalance || 0)
          step.value = 4
          ElMessage.success('支付成功，授权已生成')
        } else if (status === 'failed' || status === 'cancelled' || attempts >= 20) {
          stopPurchasePoll()
          if (status !== 'pending') ElMessage.warning('支付未完成')
        }
      } catch {
        if (attempts >= 20) stopPurchasePoll()
      }
    }, 1500)
  }

  onMounted(() => {
    fetchApps()
    fetchBalance()
    payMethod.value = 'balance'
    checkPurchaseReturn()
  })

  const typeOptionCatalog = [
    { value: 'domain', label: '单域名', icon: 'ri:global-line' },
    { value: 'wildcard', label: '泛域名', icon: 'ri:asterisk' },
    { value: 'ip', label: 'IP地址', icon: 'ri:router-line' },
    { value: 'key', label: '密钥', icon: 'ri:key-2-line' }
  ]

  const typeLabels: Record<string, string> = {
    domain: '单域名',
    wildcard: '泛域名',
    ip: 'IP地址',
    key: '密钥'
  }
  const formData = reactive({
    appId: '' as string | number,
    planId: '' as string | number,
    userId: '' as string | number,
    type: 'domain',
    domain: ''
  })

  const domainLabel = computed(() => {
    const map: Record<string, string> = {
      domain: '域名',
      wildcard: '泛域名',
      ip: 'IP地址',
      key: '密钥'
    }
    return map[formData.type] || '域名'
  })

  const domainIcon = computed(() => {
    const map: Record<string, string> = {
      domain: 'ri:global-line',
      wildcard: 'ri:asterisk',
      ip: 'ri:router-line',
      key: 'ri:key-2-line'
    }
    return map[formData.type] || 'ri:global-line'
  })

  const domainPlaceholder = computed(() => {
    const map: Record<string, string> = {
      domain: 'example.com',
      wildcard: '*.example.com',
      ip: '192.168.1.1',
      key: '系统自动生成密钥'
    }
    return map[formData.type] || ''
  })

  function validateLicenseTarget(type: string, value: string) {
    const target = (value || '').trim().toLowerCase()
    if (type === 'key') return ''
    if (type === 'domain' && !isValidSingleDomain(target)) return '单域名格式不正确'
    if (type === 'wildcard' && (!target.startsWith('*.') || !isValidSingleDomain(target.slice(2))))
      return '泛域名格式不正确'
    if (type === 'ip' && !isValidIP(target)) return 'IP 格式不正确'
    return ''
  }

  function isValidSingleDomain(value: string) {
    if (
      !value ||
      value.startsWith('*.') ||
      value.endsWith('.') ||
      /[/:@\s]/.test(value) ||
      isValidIP(value)
    )
      return false
    const labels = value.split('.')
    if (labels.length < 2) return false
    if (!/^[a-z]{2,}$/.test(labels[labels.length - 1])) return false
    return labels.every((label) => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))
  }

  function isValidIP(value: string) {
    const ipv4 = /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/
    const ipv6 = /^(([0-9a-f]{1,4}:){7}[0-9a-f]{1,4}|::1|::)$/i
    return ipv4.test(value) || ipv6.test(value)
  }

  function getTargetError() {
    if (!availableTypeOptions.value.some((option) => option.value === formData.type)) {
      return '当前应用不支持该授权类型'
    }
    const target = formData.domain.trim()
    if (formData.type !== 'key' && !target) return '请填写授权目标'
    return validateLicenseTarget(formData.type, target)
  }

  const selectedApp = computed(() => appList.value.find((a) => a.id == formData.appId))
  const availableTypeOptions = computed(() => {
    const allowedTypes = Array.isArray(selectedApp.value?.purchaseLicenseTypes)
      ? selectedApp.value.purchaseLicenseTypes
      : []
    return typeOptionCatalog.filter((option) => allowedTypes.includes(option.value))
  })
  const selectedAppPlans = computed(() => selectedApp.value?.plans || [])
  const filteredAppPlans = computed(() =>
    selectedAppPlans.value.filter(
      (plan: any) => !plan.licenseType || plan.licenseType === formData.type
    )
  )
  const selectedPlan = computed(() =>
    selectedAppPlans.value.find((p: any) => p.id == formData.planId)
  )
  const computedCost = computed(() => Number(selectedPlan.value?.price || 0))
  const originalPrice = computed(() =>
    Number(selectedPlan.value?.originalPrice ?? computedCost.value)
  )
  const agentDiscount = computed(() => Number(selectedPlan.value?.discount ?? 10))
  const basePrice = computed(() => {
    const plan = selectedPlan.value
    if (plan && Number.isFinite(Number(plan.basePrice))) return Number(plan.basePrice)
    return Math.max(0, (originalPrice.value * agentDiscount.value) / 10)
  })
  const agentDiscountAmount = computed(() => Math.max(0, originalPrice.value - basePrice.value))
  const hasAgentDiscount = computed(() => agentDiscountAmount.value >= 0.01)
  const selectedPromotion = computed(() => selectedPlan.value?.promotion || null)
  const promotionSavings = computed(() => Math.max(0, originalPrice.value - computedCost.value))
  const agentSavings = computed(() => Math.max(0, originalPrice.value - basePrice.value))
  const usePromotionPath = computed(
    () => !!selectedPromotion.value && promotionSavings.value >= agentSavings.value
  )
  const discountAmount = computed(() => Math.max(0, originalPrice.value - computedCost.value))
  const hasDiscount = computed(() => discountAmount.value >= 0.01)
  const displayTarget = computed(() =>
    formData.type === 'key' ? '系统自动生成' : formData.domain || '-'
  )
  const selectedUser = computed(() =>
    userOptions.value.find((user) => user.id === Number(formData.userId))
  )
  const ownerLabel = computed(() =>
    formData.userId
      ? selectedUser.value
        ? formatUserOption(selectedUser.value)
        : `用户 #${formData.userId}`
      : '当前代理商'
  )

  const canNext = computed(() => {
    if (step.value === 1) return !!formData.appId
    if (step.value === 2) return !!formData.planId && !getTargetError()
    return true
  })

  function minPlanPrice(app: any) {
    const prices = (app.plans || []).map((p: any) => Number(p.price || 0))
    return prices.length ? Math.min(...prices) : 0
  }

  function planHasDiscount(plan: any) {
    return Number(plan.originalPrice || 0) - Number(plan.price || 0) >= 0.01
  }

  function formatDiscount(value: number | string | undefined) {
    const discount = Number(value ?? 10)
    return `${discount.toFixed(1).replace(/\.0$/, '')}折`
  }

  function hasPromotion(app: any) {
    return (app.plans || []).some((p: any) => p.promotion)
  }

  function promoRuleText(promotion: any) {
    if (!promotion) return ''
    if (promotion.ruleType === 'discount') return `${promotion.discount} 折`
    if (promotion.ruleType === 'reduction') return '立减'
    return '固定价'
  }

  function resetPurchaseSelection() {
    Object.assign(formData, { appId: '', planId: '', userId: '', type: '', domain: '' })
    userOptions.value = []
    payMethod.value = 'balance'
    quotaInfo.value = { total: 0, used: 0, remain: 0 }
  }

  function selectApp(id: string | number) {
    const app = appList.value.find((item) => item.id == id)
    formData.appId = id
    formData.planId = ''
    formData.type = app?.purchaseLicenseTypes?.[0] || ''
    formData.domain = ''
    payMethod.value = 'balance'
    quotaInfo.value = { total: 0, used: 0, remain: 0 }
  }

  function selectPlan(id: string | number) {
    formData.planId = id
  }

  function selectType(type: string) {
    formData.type = type
    if (type === 'key') {
      formData.domain = ''
    }
    if (formData.planId) {
      const selected = selectedAppPlans.value.find((p: any) => p.id == formData.planId)
      if (selected?.licenseType && selected.licenseType !== type) {
        formData.planId = ''
      }
    }
  }

  function goStep(target: number) {
    if (target >= step.value) return
    step.value = target
  }

  function formatDuration(days: number | string | undefined) {
    const value = Number(days || 0)
    return value === 0 ? '永久' : `${value}天`
  }

  async function copyLicenseNo() {
    const licenseNo = purchaseResult.value?.licenseNo
    if (!licenseNo) return
    try {
      await navigator.clipboard.writeText(licenseNo)
      ElMessage.success('授权编号已复制')
    } catch {
      ElMessage.warning('复制失败，请手动复制')
    }
  }

  function resetFlow() {
    purchaseResult.value = null
    step.value = 1
    resetPurchaseSelection()
    fetchApps()
    fetchBalance()
  }

  function handleNext() {
    if (step.value === 1 && !formData.appId) {
      ElMessage.warning('请先选择应用')
      return
    }
    if (step.value === 2) {
      if (!formData.planId) {
        ElMessage.warning('请选择套餐')
        return
      }
      const targetError = getTargetError()
      if (targetError) {
        ElMessage.warning(targetError)
        return
      }
    }
    step.value++
    if (step.value === 3) {
      fetchPayOptions()
    }
  }

  const isOnlinePay = computed(() => payMethod.value !== 'balance' && payMethod.value !== 'quota')

  const purchaseButtonText = computed(() => {
    if (purchasing.value) {
      return payMethod.value === 'balance' || payMethod.value === 'quota'
        ? '正在生成授权...'
        : '正在创建支付订单...'
    }
    if (payMethod.value === 'quota') return '使用配额支付（消耗 1 配额）'
    return `确认支付 ¥${computedCost.value.toFixed(2)}`
  })

  async function handlePurchase() {
    if (purchasing.value) return
    if (isOnlinePay.value && computedCost.value <= 0) {
      ElMessage.warning('0 元套餐请使用余额或配额支付')
      return
    }
    if (payMethod.value === 'balance' && agentBalance.value < computedCost.value) {
      ElMessage.warning('余额不足')
      return
    }
    const targetError = getTargetError()
    if (targetError) {
      ElMessage.warning(targetError)
      return
    }
    purchasing.value = true
    try {
      const { data } = await axios.post(
        '/api/agent-panel/purchase',
        {
          appId: Number(formData.appId),
          planId: Number(formData.planId),
          userId: formData.userId ? Number(formData.userId) : 0,
          type: formData.type,
          domain: formData.domain,
          payMethod: payMethod.value
        },
        { headers: authHeaders.value }
      )
      if (data.code === 200) {
        if (data.data?.payUrl) {
          ElMessage.success('支付订单已创建，正在跳转收银台')
          window.location.href = data.data.payUrl
          return
        }
        purchaseResult.value = data.data
        if (data.data?.payMethod === 'quota') {
          quotaInfo.value.remain = Math.max(0, quotaInfo.value.remain - 1)
        } else {
          agentBalance.value = Number(data.data.newBalance || 0)
        }
        step.value = 4
        ElMessage.success('开通成功，授权已生成')
      } else {
        ElMessage.error(data.msg || '开通失败')
      }
    } catch {
      ElMessage.error('请求失败，请重试')
    } finally {
      purchasing.value = false
    }
  }
</script>

<style scoped lang="scss">
  .agent-purchase {
    max-width: 960px;
    margin: 0 auto;
  }

  .steps-bar {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px 0 32px;

    .step {
      display: flex;
      gap: 8px;
      align-items: center;
      cursor: pointer;

      .step-dot {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 28px;
        height: 28px;
        font-size: 12px;
        font-weight: 700;
        color: var(--el-text-color-secondary);
        background: var(--el-fill-color);
        border-radius: 50%;
        transition: all 0.3s;
      }

      .step-text {
        font-size: 13px;
        font-weight: 500;
        color: var(--el-text-color-secondary);
        transition: color 0.3s;
      }

      &.active .step-dot {
        color: #fff;
        background: var(--el-color-primary);
        box-shadow: 0 2px 8px rgb(64 158 255 / 30%);
      }

      &.active .step-text {
        color: var(--el-color-primary);
      }

      &.done .step-dot {
        color: #fff;
        background: var(--el-color-success);
      }
    }

    .step-line {
      width: 60px;
      height: 2px;
      margin: 0 12px;
      background: var(--el-fill-color);
      border-radius: 1px;
      transition: background 0.3s;

      &.active {
        background: var(--el-color-primary);
      }
    }
  }

  .fade-enter-active,
  .fade-leave-active {
    transition:
      opacity 0.2s,
      transform 0.2s;
  }

  .fade-enter-from {
    opacity: 0;
    transform: translateX(12px);
  }

  .fade-leave-to {
    opacity: 0;
    transform: translateX(-12px);
  }

  .empty-card {
    padding: 40px 24px;
    color: var(--el-text-color-secondary);
    text-align: center;
    background: var(--el-bg-color);
    border: 1px dashed var(--el-border-color);
    border-radius: 16px;
  }

  .app-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 14px;
  }

  .app-card {
    position: relative;
    padding: 20px 18px 16px;
    overflow: hidden;
    text-align: center;
    cursor: pointer;
    background: linear-gradient(180deg, var(--el-bg-color) 0%, var(--el-bg-color-page) 100%);
    border: 1.5px solid var(--el-border-color-lighter);
    border-radius: 18px;
    transition: all 0.25s ease;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      box-shadow: 0 12px 30px rgb(15 23 42 / 8%);
      transform: translateY(-3px);
    }

    &.active {
      background: linear-gradient(
        135deg,
        var(--el-color-primary-light-9) 0%,
        var(--el-bg-color) 100%
      );
      border-color: var(--el-color-primary);
      box-shadow: 0 10px 26px rgb(64 158 255 / 16%);
    }

    &.has-promo {
      background: linear-gradient(
        180deg,
        var(--el-color-danger-light-9) 0%,
        var(--el-bg-color) 70%
      );
      border-color: var(--el-color-danger-light-5);
    }
  }

  .app-promo-badge {
    position: absolute;
    top: 10px;
    right: 10px;
    display: inline-flex;
    gap: 4px;
    align-items: center;
    padding: 3px 8px;
    font-size: 10px;
    font-weight: 700;
    color: #fff;
    background: linear-gradient(135deg, #f43f5e, #e11d48);
    border-radius: 999px;
    box-shadow: 0 4px 10px rgb(225 29 72 / 22%);
  }

  .app-icon-wrap {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 50px;
    height: 50px;
    margin: 0 auto 10px;
    color: var(--el-color-primary);
    background: linear-gradient(
      135deg,
      var(--el-color-primary-light-8),
      var(--el-color-primary-light-9)
    );
    border-radius: 13px;
    box-shadow: inset 0 1px 0 rgb(255 255 255 / 60%);
  }

  .app-name {
    margin-bottom: 4px;
    font-size: 14px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  .app-desc {
    margin-bottom: 12px;
    overflow: hidden;
    font-size: 11px;
    color: var(--el-text-color-secondary);
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .app-footer {
    display: flex;
    gap: 8px;
    align-items: baseline;
    justify-content: space-between;
    padding-top: 8px;
    border-top: 1px dashed var(--el-border-color-lighter);
  }

  .app-pricing {
    display: flex;
    gap: 3px;
    align-items: baseline;

    .price-currency {
      font-size: 12px;
      font-weight: 700;
      color: var(--el-color-primary);
    }

    .price-amount {
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      font-size: 21px;
      font-weight: 800;
      color: var(--el-color-primary);
    }

    .price-unit {
      font-size: 11px;
      color: var(--el-text-color-secondary);
    }
  }

  .app-check-icon {
    position: absolute;
    top: 12px;
    left: 12px;
    color: var(--el-color-primary);
  }

  .plan-count {
    margin-top: 2px;
    font-size: 12px;
    font-weight: 600;
    color: var(--el-text-color-secondary);
  }

  .section-title {
    display: flex;
    gap: 8px;
    align-items: center;
    margin-bottom: 16px;
    font-size: 16px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  .plan-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 14px;
  }

  .plan-card {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 12px;
    padding: 20px 18px;
    overflow: hidden;
    cursor: pointer;
    background: var(--el-bg-color);
    border: 1.5px solid var(--el-border-color-lighter);
    border-radius: 16px;
    transition: all 0.22s ease;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      box-shadow: 0 10px 26px rgb(15 23 42 / 6%);
      transform: translateY(-2px);
    }

    &.active {
      background: linear-gradient(
        180deg,
        var(--el-color-primary-light-9) 0%,
        var(--el-bg-color) 100%
      );
      border-color: var(--el-color-primary);
      box-shadow: 0 8px 22px rgb(64 158 255 / 14%);
    }

    &.has-promo {
      background: linear-gradient(
        180deg,
        var(--el-color-danger-light-9) 0%,
        var(--el-bg-color) 70%
      );
      border-color: var(--el-color-danger-light-5);

      &.active {
        background: linear-gradient(
          180deg,
          var(--el-color-danger-light-8) 0%,
          var(--el-color-danger-light-9) 100%
        );
        border-color: var(--el-color-danger);
      }
    }
  }

  .promo-badge {
    position: absolute;
    top: 14px;
    right: 14px;
    padding: 4px 10px;
    font-size: 11px;
    font-weight: 700;
    color: #fff;
    background: linear-gradient(135deg, #f43f5e, #e11d48);
    border-radius: 999px;
    box-shadow: 0 6px 14px rgb(225 29 72 / 22%);
  }

  .plan-head {
    display: flex;
    gap: 10px;
    align-items: center;
    justify-content: space-between;
    min-height: 22px;
  }

  .plan-name {
    font-size: 15px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  .plan-tag {
    padding: 4px 8px;
    font-size: 11px;
    font-weight: 700;
    color: var(--el-color-primary);
    background: var(--el-color-primary-light-9);
    border-radius: 999px;
  }

  .plan-tag.promo {
    color: var(--el-color-danger);
    background: var(--el-color-danger-light-8);
  }

  .plan-pricing {
    display: flex;
    gap: 6px;
    align-items: baseline;
  }

  .plan-currency {
    font-size: 14px;
    font-weight: 700;
    color: var(--el-color-primary);
  }

  .plan-amount {
    font-family: 'DIN Alternate', 'Roboto Mono', monospace;
    font-size: 26px;
    font-weight: 800;
    color: var(--el-color-primary);
  }

  .plan-card.has-promo .plan-amount,
  .plan-card.has-promo .plan-currency {
    color: var(--el-color-danger);
  }

  .plan-original {
    margin-left: auto;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    text-decoration: line-through;
  }

  .plan-meta {
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .plan-duration {
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  .plan-time {
    line-height: 1.4;
    color: var(--el-text-color-secondary);
  }

  .config-layout {
    display: flex;
    gap: 24px;
  }

  .config-main {
    flex: 1;
  }

  .config-section {
    margin-bottom: 28px;
  }

  .config-label {
    display: flex;
    gap: 5px;
    align-items: center;
    margin-bottom: 12px;
    font-size: 13px;
    font-weight: 600;
    color: var(--el-text-color-primary);
  }

  .config-hint {
    display: block;
    margin-top: 8px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .type-options {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .type-chip {
    display: flex;
    gap: 6px;
    align-items: center;
    padding: 8px 16px;
    font-size: 13px;
    font-weight: 500;
    color: var(--el-text-color-regular);
    cursor: pointer;
    border: 1.5px solid var(--el-border-color-lighter);
    border-radius: 20px;
    transition: all 0.2s;

    &:hover {
      color: var(--el-color-primary);
      border-color: var(--el-color-primary-light-5);
    }

    &.active {
      color: var(--el-color-primary);
      background: var(--el-color-primary-light-9);
      border-color: var(--el-color-primary);
    }
  }

  .owner-select,
  .domain-input-wrap {
    max-width: 380px;
  }

  .owner-select {
    width: 100%;
  }

  .domain-input-wrap {
    position: relative;

    .input-icon {
      position: absolute;
      top: 50%;
      left: 14px;
      color: var(--el-text-color-placeholder);
      transform: translateY(-50%);
    }

    .domain-input {
      width: 100%;
      height: 40px;
      padding: 0 14px 0 40px;
      font-size: 14px;
      background: var(--el-bg-color);
      border: 1.5px solid var(--el-border-color);
      border-radius: 10px;
      outline: none;
      transition: all 0.2s;

      &:focus {
        border-color: var(--el-color-primary);
        box-shadow: 0 0 0 3px rgb(64 158 255 / 10%);
      }

      &:disabled {
        cursor: not-allowed;
        background: var(--el-fill-color-lighter);
      }
    }
  }

  .config-preview {
    flex-shrink: 0;
    width: 280px;
  }

  .preview-card {
    overflow: hidden;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 16px;
    box-shadow: 0 2px 12px rgb(0 0 0 / 4%);

    .preview-header {
      display: flex;
      gap: 8px;
      align-items: center;
      padding: 16px 20px;
      font-size: 14px;
      font-weight: 600;
      color: var(--el-color-primary);
      background: var(--el-color-primary-light-9);
    }

    .preview-body {
      padding: 16px 20px;

      .preview-item {
        display: flex;
        gap: 12px;
        justify-content: space-between;
        padding: 6px 0;

        .preview-key {
          font-size: 12px;
          color: var(--el-text-color-secondary);
        }

        .preview-val {
          font-size: 12px;
          font-weight: 500;
          color: var(--el-text-color-primary);
          text-align: right;

          &.mono {
            font-family: 'Roboto Mono', monospace;
            font-size: 11px;
          }
        }
      }
    }

    .preview-footer {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 14px 20px;
      border-top: 1px dashed var(--el-border-color-lighter);

      .preview-total-label {
        font-size: 13px;
        font-weight: 600;
      }

      .preview-total-price {
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
        font-size: 20px;
        font-weight: 800;
        color: var(--el-color-primary);
      }

      .preview-price-wrap {
        display: flex;
        gap: 8px;
        align-items: baseline;
      }

      .preview-original-price {
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
        font-size: 12px;
        color: var(--el-text-color-placeholder);
        text-decoration: line-through;
      }
    }
  }

  .payment-layout {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .payment-detail-card,
  .payment-action-card {
    padding: 24px;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 16px;
    box-shadow: 0 2px 12px rgb(0 0 0 / 4%);
  }

  .payment-detail-card {
    flex: 1;

    .payment-header {
      display: flex;
      gap: 8px;
      align-items: center;
      margin-bottom: 20px;
      font-size: 15px;
      font-weight: 600;
      color: var(--el-text-color-primary);
    }

    .payment-row {
      display: flex;
      justify-content: space-between;
      padding: 10px 0;
      font-size: 13px;
      border-bottom: 1px solid var(--el-border-color-extra-light);

      span:first-child {
        color: var(--el-text-color-secondary);
      }

      span:last-child {
        font-weight: 500;
        color: var(--el-text-color-primary);
      }

      .mono {
        font-family: 'Roboto Mono', monospace;
      }

      &.discount-row span:last-child {
        font-weight: 600;
        color: var(--el-color-danger);
      }
    }

    .payment-total-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding-top: 16px;
      margin-top: 4px;

      span:first-child {
        font-size: 14px;
        font-weight: 600;
      }

      .payment-price {
        .price-sym {
          font-size: 16px;
          font-weight: 600;
          color: var(--el-color-primary);
        }

        .price-num {
          font-family: 'DIN Alternate', 'Roboto Mono', monospace;
          font-size: 28px;
          font-weight: 800;
          color: var(--el-color-primary);
        }
      }
    }
  }

  .payment-action-card {
    display: flex;
    flex-direction: column;
    gap: 20px;
    width: 100%;

    .method-options {
      display: flex;
      gap: 10px;
    }

    .method-item {
      display: flex;
      flex: 1;
      flex-direction: column;
      gap: 6px;
      align-items: center;
      justify-content: center;
      padding: 12px;
      font-size: 13px;
      font-weight: 500;
      cursor: pointer;
      border: 1.5px solid var(--el-border-color-lighter);
      border-radius: 10px;
      transition: all 0.2s;

      .method-balance {
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
        font-size: 11px;
        color: var(--el-color-success);
      }

      &.active {
        background: var(--el-color-primary-light-9);
        border-color: var(--el-color-primary);
      }
    }
  }

  .balance-warning {
    padding: 10px 12px;
    font-size: 13px;
    color: var(--el-color-danger);
    text-align: center;
    background: var(--el-color-danger-light-9);
    border-radius: 10px;
  }

  .purchase-btn {
    display: flex;
    gap: 8px;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 48px;
    font-size: 15px;
    font-weight: 700;
    color: #fff;
    cursor: pointer;
    background: linear-gradient(135deg, #409eff 0%, #2563eb 100%);
    border: none;
    border-radius: 12px;
    box-shadow: 0 4px 14px rgb(64 158 255 / 35%);
    transition: all 0.25s;

    &:hover {
      box-shadow: 0 6px 20px rgb(64 158 255 / 45%);
      transform: translateY(-1px);
    }

    &:disabled {
      cursor: not-allowed;
      box-shadow: none;
      opacity: 0.55;
      transform: none;
    }
  }

  .payment-hint {
    display: flex;
    gap: 4px;
    align-items: center;
    justify-content: center;
    font-size: 11px;
    color: var(--el-text-color-placeholder);
  }

  .success-card {
    max-width: 520px;
    margin: 0 auto;
    overflow: hidden;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 16px;
    box-shadow: 0 8px 28px rgb(0 0 0 / 6%);

    .success-banner {
      display: flex;
      flex-direction: column;
      gap: 6px;
      align-items: center;
      padding: 28px 32px 22px;
      text-align: center;
      background: linear-gradient(
        160deg,
        var(--el-color-success-light-8) 0%,
        var(--el-bg-color) 100%
      );

      .success-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 52px;
        height: 52px;
        color: #fff;
        background: var(--el-color-success);
        border-radius: 50%;
        box-shadow: 0 6px 16px rgb(103 194 58 / 35%);
      }

      h3 {
        margin: 8px 0 0;
        font-size: 20px;
        color: var(--el-text-color-primary);
      }

      .success-sub {
        margin: 0;
        font-size: 13px;
        color: var(--el-text-color-secondary);
      }
    }

    .success-license-no {
      display: flex;
      gap: 10px;
      align-items: center;
      justify-content: center;
      padding: 12px 16px;
      margin: 0 32px;
      background: var(--el-fill-color-light);
      border-radius: 10px;

      .license-label {
        font-size: 12px;
        color: var(--el-text-color-secondary);
        white-space: nowrap;
      }

      .license-value {
        font-size: 14px;
        font-weight: 600;
        color: var(--el-text-color-primary);
        word-break: break-all;
      }

      .copy-btn {
        display: flex;
        flex-shrink: 0;
        align-items: center;
        justify-content: center;
        width: 26px;
        height: 26px;
        color: var(--el-text-color-secondary);
        cursor: pointer;
        background: transparent;
        border: none;
        border-radius: 6px;
        transition: all 0.2s;

        &:hover {
          color: var(--el-color-primary);
          background: var(--el-color-primary-light-8);
        }
      }
    }

    .success-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 1px;
      margin: 20px 32px 0;
      overflow: hidden;
      background: var(--el-border-color-extra-light);
      border: 1px solid var(--el-border-color-extra-light);
      border-radius: 10px;

      .success-item {
        display: flex;
        flex-direction: column;
        gap: 4px;
        padding: 12px 16px;
        background: var(--el-bg-color);

        .item-label {
          font-size: 12px;
          color: var(--el-text-color-secondary);
        }

        .item-value {
          font-size: 14px;
          font-weight: 600;
          color: var(--el-text-color-primary);
        }
      }
    }

    .success-amount {
      display: flex;
      align-items: center;
      padding: 14px 20px;
      margin: 20px 32px 0;
      background: var(--el-color-primary-light-9);
      border-radius: 10px;

      .amount-cell {
        display: flex;
        flex: 1;
        flex-direction: column;
        gap: 2px;
        align-items: center;

        .amount-label {
          font-size: 12px;
          color: var(--el-text-color-secondary);
        }

        .amount-value {
          font-size: 18px;
          font-weight: 700;
          color: var(--el-text-color-primary);

          &.free {
            color: var(--el-color-success);
          }
        }
      }

      .amount-divider {
        width: 1px;
        height: 32px;
        background: var(--el-border-color-light);
      }
    }

    .success-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      margin: 24px auto 28px;
    }
  }

  @media (width <= 640px) {
    .success-card {
      .success-banner {
        padding: 22px 20px 18px;
      }

      .success-license-no {
        margin: 0 20px;
      }

      .success-grid {
        margin: 16px 20px 0;
      }

      .success-amount {
        margin: 16px 20px 0;
      }
    }
  }

  .bottom-nav {
    display: flex;
    gap: 12px;
    justify-content: center;
    padding: 24px 0;
  }

  @media (width <= 768px) {
    .app-grid {
      grid-template-columns: 1fr;
    }

    .config-layout {
      flex-direction: column;
    }

    .config-preview {
      width: 100%;
    }

    .plan-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
