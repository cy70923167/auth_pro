<template>
  <div class="user-purchase">
    <!-- 步骤指示器 -->
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

    <!-- Step 1: 选择应用 -->
    <transition name="fade" mode="out-in">
      <div v-if="step === 1" key="step1" class="step-content">
        <div v-if="appList.length === 0" class="empty-card">
          暂无可购买应用，请联系管理员先启用应用和套餐
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
                    <span v-if="plan.promotion" class="plan-tag promo">{{ promoRuleText(plan.promotion) }}</span>
                  </div>
                  <div class="plan-pricing">
                    <span class="plan-currency">¥</span>
                    <span class="plan-amount">{{ Number(plan.price).toFixed(2) }}</span>
                    <span v-if="plan.promotion" class="plan-original">¥{{ Number(plan.originalPrice).toFixed(2) }}</span>
                  </div>
                  <div class="plan-meta">
                    <span class="plan-duration">{{ plan.durationText }}</span>
                    <span v-if="plan.promotion" class="plan-time">活动截止：{{ plan.promotion.endsAt }}</span>
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
                <span class="preview-total-label">套餐价格</span>
                <span class="preview-total-price">¥{{ computedCost.toFixed(2) }}</span>
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
                <span>授权类型</span><span>{{ typeLabels[formData.type] }}</span>
              </div>
              <div class="payment-row">
                <span>授权目标</span><span class="mono">{{ displayTarget }}</span>
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
                  :class="{
                    active: payMethod === option.code,
                    'balance-method': option.code === 'balance'
                  }"
                  @click="payMethod = option.code"
                >
                  <iconify-icon :icon="option.icon" width="24" :color="option.color" />
                  <span>{{ option.label }}</span>
                  <template v-if="option.code === 'balance'">
                    <span class="method-balance">¥{{ userBalance.toFixed(2) }}</span>
                    <button
                      class="method-recharge-btn"
                      type="button"
                      @click.stop="openRechargeDialog"
                    >
                      充值余额
                    </button>
                  </template>
                </div>
              </div>
            </div>
            <div
              class="balance-warning"
              v-if="payMethod === 'balance' && userBalance < computedCost"
            >
              <span>当前余额不足，无法完成购买</span>
              <button type="button" @click="openRechargeDialog">立即充值</button>
            </div>
            <button
              class="purchase-btn"
              :disabled="
                purchasing ||
                (payMethod === 'balance' && userBalance < computedCost) ||
                (isOnlinePay && computedCost <= 0)
              "
              @click="handlePurchase"
            >
              <iconify-icon icon="ri:secure-payment-line" width="20" />
              <span>{{ purchaseButtonText }}</span>
            </button>
            <p class="payment-hint">
              <iconify-icon icon="ri:shield-check-line" width="14" />
              <template v-if="payMethod === 'balance'">
                套餐价格以后端为准，余额扣款后授权即时生效
              </template>
              <template v-else>支付成功后授权即时生效</template>
            </p>
          </div>
        </div>
      </div>

      <div v-else-if="step === 4" key="step4" class="step-content">
        <div class="success-card">
          <div class="success-hero">
            <div class="success-icon-wrap">
              <iconify-icon icon="ri:checkbox-circle-fill" width="34" />
            </div>
            <div class="success-title-block">
              <h3>授权已生成</h3>
              <p>购买完成，授权已经即时生效</p>
            </div>
          </div>

          <div class="license-no-card">
            <span class="license-no-label">授权编号</span>
            <span class="license-no-value">{{ purchaseResult?.licenseNo }}</span>
          </div>

          <div class="success-info-grid">
            <div class="success-info-item">
              <span class="info-label">应用</span>
              <span class="info-value">{{ purchaseResult?.appName }}</span>
            </div>
            <div class="success-info-item">
              <span class="info-label">套餐</span>
              <span class="info-value">{{ purchaseResult?.planName }}</span>
            </div>
            <div class="success-info-item">
              <span class="info-label">授权时长</span>
              <span class="info-value">{{ formatDuration(purchaseResult?.durationDays) }}</span>
            </div>
            <div class="success-info-item">
              <span class="info-label">剩余余额</span>
              <span class="info-value">¥{{ userBalance.toFixed(2) }}</span>
            </div>
          </div>

          <div class="success-amount-card">
            <span>本次扣款</span>
            <strong>¥{{ Number(purchaseResult?.cost || 0).toFixed(2) }}</strong>
          </div>

          <div class="success-actions">
            <el-button size="large" @click="goToLicenses">查看授权</el-button>
            <el-button type="primary" size="large" @click="resetFlow">继续购买</el-button>
          </div>
        </div>
      </div>
    </transition>

    <div class="bottom-nav" v-if="step < 4">
      <el-button v-if="step > 1" @click="step--">上一步</el-button>
      <el-button v-if="step < 3" type="primary" :disabled="!canNext" @click="handleNext">
        下一步
      </el-button>
    </div>

    <el-dialog v-model="rechargeDialog.visible" title="余额充值" width="420px" append-to-body>
      <div class="recharge-dialog-body">
        <label class="config-label">充值金额</label>
        <el-input-number
          v-model="rechargeDialog.amount"
          :min="0.01"
          :max="1000000"
          :precision="2"
          :step="10"
          controls-position="right"
          class="recharge-amount-input"
        />
        <div class="quick-amounts">
          <button
            v-for="amount in quickRechargeAmounts"
            :key="amount"
            type="button"
            @click="rechargeDialog.amount = amount"
          >
            ¥{{ amount }}
          </button>
        </div>

        <label class="config-label recharge-pay-label">支付方式</label>
        <el-radio-group
          v-if="rechargeOptions.payTypes.length > 0"
          v-model="rechargeDialog.payType"
          class="recharge-pay-types"
        >
          <el-radio-button
            v-for="payType in rechargeOptions.payTypes"
            :key="payType"
            :label="payType"
          >
            {{ payTypeLabels[payType] || payType }}
          </el-radio-button>
        </el-radio-group>
        <p v-else class="recharge-pay-empty">支付通道未开启，请联系管理员</p>
        <p class="recharge-tip">支付成功后余额自动入账，再用余额购买授权。</p>
      </div>
      <template #footer>
        <el-button @click="rechargeDialog.visible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="rechargeDialog.submitting"
          :disabled="rechargeOptions.payTypes.length === 0"
          @click="submitRecharge"
        >
          去支付
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { ref, reactive, computed, onMounted } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { ElMessage } from 'element-plus'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import axios from 'axios'

  const router = useRouter()
  const route = useRoute()
  const step = ref(1)
  const payMethod = ref('balance')
  const payOptions = ref<any[]>([
    { code: 'balance', label: '余额支付', icon: 'ri:wallet-3-line', color: '#2e7d32' }
  ])
  const userBalance = ref(0)
  const purchasing = ref(false)
  const purchaseResult = ref<any>(null)
  const rechargeDialog = reactive({
    visible: false,
    submitting: false,
    amount: 50,
    payType: 'alipay'
  })
  const payTypeLabels: Record<string, string> = {
    alipay: '支付宝',
    wxpay: '微信',
    qqpay: 'QQ支付'
  }
  const rechargeOptions = reactive({
    payTypes: [] as string[],
    defaultType: 'alipay'
  })
  const quickRechargeAmounts = [10, 30, 50, 100]
  const rechargeOrderStorageKey = 'user_panel_recharge_order'
  const purchaseOrderStorageKey = 'user_panel_purchase_order'

  const stepItems = [
    { value: 1, label: '选择应用' },
    { value: 2, label: '授权配置' },
    { value: 3, label: '支付购买' },
    { value: 4, label: '生成授权' }
  ]

  function getToken() {
    return localStorage.getItem('user_panel_token') || ''
  }

  const authHeaders = computed(() => ({ Authorization: `Bearer ${getToken()}` }))

  const appList = ref<any[]>([])

  async function fetchApps() {
    try {
      const { data } = await axios.get('/api/user-panel/apps/purchase', {
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
            price: normalizePrice(p)
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
      }
    } catch {
      ElMessage.error('加载可购买应用失败')
    }
  }

  function normalizePrice(plan: any) {
    return Number(plan.price ?? plan.Price ?? plan.amount ?? plan.Amount ?? 0)
  }

  async function fetchPayOptions() {
    try {
      const { data } = await axios.get('/api/user-panel/purchase/pay-options', {
        headers: authHeaders.value
      })
      if (data.code === 200) {
        const options =
          Array.isArray(data.data?.options) && data.data.options.length > 0
            ? data.data.options
            : [
                {
                  code: 'balance',
                  label: '余额支付',
                  icon: 'ri:wallet-3-line',
                  color: '#2e7d32'
                }
              ]
        payOptions.value = options
        if (!options.some((option: any) => option.code === payMethod.value)) {
          payMethod.value = options[0]?.code || 'balance'
        }
      }
    } catch {
      ElMessage.error('加载支付方式失败')
    }
  }

  async function fetchBalance() {
    try {
      const { data } = await axios.get('/api/user-panel/balance', { headers: authHeaders.value })
      if (data.code === 200) {
        userBalance.value = Number(data.data.balance || 0)
      }
    } catch {
      ElMessage.error('加载余额失败')
    }
  }

  function notifyBalanceRefresh() {
    window.dispatchEvent(new CustomEvent('user-panel-balance-refresh'))
  }

  async function fetchRechargeOptions() {
    try {
      const { data } = await axios.get('/api/user-panel/recharge/options', {
        headers: authHeaders.value
      })
      if (data.code === 200) {
        rechargeOptions.payTypes = Array.isArray(data.data?.payTypes) ? data.data.payTypes : []
        rechargeOptions.defaultType = data.data?.defaultType || 'alipay'
        if (!rechargeOptions.payTypes.includes(rechargeDialog.payType)) {
          rechargeDialog.payType = rechargeOptions.payTypes.includes(rechargeOptions.defaultType)
            ? rechargeOptions.defaultType
            : rechargeOptions.payTypes[0] || rechargeOptions.defaultType
        }
      }
    } catch {
      ElMessage.error('加载充值方式失败')
    }
  }

  onMounted(() => {
    fetchApps()
    fetchBalance()
    fetchPayOptions()
    fetchRechargeOptions()
    handlePurchaseReturn()
    handleRechargeReturn()
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
  const isOnlinePay = computed(() => payMethod.value !== 'balance')
  const purchaseButtonText = computed(() => {
    if (purchasing.value) {
      return isOnlinePay.value ? '正在创建支付订单...' : '正在生成授权...'
    }
    return `确认支付 ¥${computedCost.value.toFixed(2)}`
  })
  const displayTarget = computed(() =>
    formData.type === 'key' ? '系统自动生成' : formData.domain || '-'
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

  function hasPromotion(app: any) {
    return (app.plans || []).some((p: any) => p.promotion)
  }

  function resetPurchaseSelection() {
    Object.assign(formData, { appId: '', planId: '', type: '', domain: '' })
  }

  function selectApp(id: string | number) {
    const app = appList.value.find((item) => item.id == id)
    formData.appId = id
    formData.planId = ''
    formData.type = app?.purchaseLicenseTypes?.[0] || ''
    formData.domain = ''
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

  function promoRuleText(promotion: any) {
    if (!promotion) return ''
    if (promotion.ruleType === 'discount') return `${promotion.discount} 折`
    if (promotion.ruleType === 'reduction') return '立减'
    return '固定价'
  }

  function goToLicenses() {
    router.push('/user/licenses')
  }

  function resetFlow() {
    purchaseResult.value = null
    step.value = 1
    resetPurchaseSelection()
    fetchApps()
    fetchBalance()
  }

  function openRechargeDialog() {
    rechargeDialog.visible = true
    fetchRechargeOptions()
  }

  async function submitRecharge() {
    if (rechargeDialog.submitting) return
    const amount = Number(rechargeDialog.amount)
    if (!Number.isFinite(amount) || amount < 0.01) {
      ElMessage.warning('充值金额不能低于 ¥0.01')
      return
    }

    rechargeDialog.submitting = true
    try {
      const { data } = await axios.post(
        '/api/user-panel/recharge/orders',
        {
          amount,
          payType: rechargeDialog.payType
        },
        { headers: authHeaders.value }
      )

      if (data.code === 200) {
        const orderNo = data.data?.orderNo || ''
        const payUrl = data.data?.payUrl || ''
        if (!payUrl) {
          ElMessage.error('支付地址生成失败')
          return
        }
        if (orderNo) sessionStorage.setItem(rechargeOrderStorageKey, orderNo)
        window.location.href = payUrl
        return
      }
      ElMessage.error(data.msg || '创建充值订单失败')
    } catch {
      ElMessage.error('创建充值订单失败，请重试')
    } finally {
      rechargeDialog.submitting = false
    }
  }

  async function handleRechargeReturn() {
    const queryOrderNo =
      typeof route.query.rechargeOrder === 'string' ? route.query.rechargeOrder : ''
    const storedOrderNo = sessionStorage.getItem(rechargeOrderStorageKey) || ''
    const orderNo = queryOrderNo || storedOrderNo
    if (!orderNo || orderNo.startsWith('LP')) return

    const paid = await pollRechargeOrder(orderNo)
    if (paid) {
      sessionStorage.removeItem(rechargeOrderStorageKey)
      clearRechargeReturnQuery()
      await fetchBalance()
      notifyBalanceRefresh()
      ElMessage.success('充值已到账，余额已刷新')
    }
  }

  function clearRechargeReturnQuery() {
    if (!route.query.rechargeOrder && !route.query.rechargeReturn) return
    const nextQuery = { ...route.query }
    delete nextQuery.rechargeOrder
    delete nextQuery.rechargeReturn
    router.replace({ path: route.path, query: nextQuery })
  }

  async function pollRechargeOrder(orderNo: string) {
    for (let index = 0; index < 6; index++) {
      const paid = await fetchRechargeOrderStatus(orderNo)
      if (paid) return true
      await wait(2000)
    }
    ElMessage.info('支付结果处理中，稍后可刷新页面查看余额')
    return false
  }

  async function fetchRechargeOrderStatus(orderNo: string) {
    try {
      const { data } = await axios.get(
        `/api/user-panel/recharge/orders/${encodeURIComponent(orderNo)}`,
        {
          headers: authHeaders.value
        }
      )
      if (data.code !== 200) return false
      if (data.data?.status === 'paid') {
        userBalance.value = Number(data.data.balance || userBalance.value)
        return true
      }
    } catch {
      return false
    }
    return false
  }

  async function handlePurchaseReturn() {
    const queryOrderNo =
      typeof route.query.rechargeOrder === 'string' ? route.query.rechargeOrder : ''
    const storedOrderNo = sessionStorage.getItem(purchaseOrderStorageKey) || ''
    const orderNo = queryOrderNo || storedOrderNo
    if (!orderNo || !orderNo.startsWith('UP')) return

    const result = await pollPurchaseOrder(orderNo)
    if (!result) return

    purchaseResult.value = {
      licenseNo: result.licenseNo,
      licenseId: result.licenseId,
      orderNo,
      payMethod: result.payMethod,
      appName: result.appName,
      planName: result.planName,
      durationDays: result.durationDays,
      cost: Number(result.cost || 0)
    }
    userBalance.value = Number(result.newBalance || userBalance.value)
    sessionStorage.removeItem(purchaseOrderStorageKey)
    clearPurchaseReturnQuery()
    notifyBalanceRefresh()
    step.value = 4
    ElMessage.success('支付成功，授权已生成')
  }

  async function pollPurchaseOrder(orderNo: string) {
    for (let index = 0; index < 20; index++) {
      const result = await fetchPurchaseOrderStatus(orderNo)
      if (result?.status === 'paid') return result
      if (result?.status === 'failed' || result?.status === 'cancelled') {
        sessionStorage.removeItem(purchaseOrderStorageKey)
        clearPurchaseReturnQuery()
        ElMessage.warning('支付未完成')
        return null
      }
      await wait(1500)
    }
    ElMessage.info('支付结果处理中，稍后可刷新页面继续确认')
    return null
  }

  async function fetchPurchaseOrderStatus(orderNo: string) {
    try {
      const { data } = await axios.get(
        `/api/user-panel/purchase/orders/${encodeURIComponent(orderNo)}`,
        {
          headers: authHeaders.value
        }
      )
      return data.code === 200 ? data.data : null
    } catch {
      return null
    }
  }

  function clearPurchaseReturnQuery() {
    if (!route.query.rechargeOrder && !route.query.rechargeReturn) return
    const nextQuery = { ...route.query }
    delete nextQuery.rechargeOrder
    delete nextQuery.rechargeReturn
    router.replace({ path: route.path, query: nextQuery })
  }

  function wait(ms: number) {
    return new Promise((resolve) => window.setTimeout(resolve, ms))
  }

  function handleNext() {
    if (step.value === 1 && !formData.appId) {
      ElMessage.warning('请先选择应用')
      return
    }
    if (step.value === 2) {
      const targetError = getTargetError()
      if (targetError) {
        ElMessage.warning(targetError)
        return
      }
    }
    if (step.value === 2 && !formData.planId) {
      ElMessage.warning('请选择套餐')
      return
    }
    step.value++
    if (step.value === 3) {
      fetchPayOptions()
    }
  }

  async function handlePurchase() {
    if (purchasing.value) return
    if (isOnlinePay.value && computedCost.value <= 0) {
      ElMessage.warning('0 元套餐请使用余额支付')
      return
    }
    if (payMethod.value === 'balance' && userBalance.value < computedCost.value) {
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
        '/api/user-panel/purchase',
        {
          appId: Number(formData.appId),
          planId: Number(formData.planId),
          type: formData.type,
          domain: formData.domain,
          payMethod: payMethod.value
        },
        { headers: authHeaders.value }
      )
      if (data.code === 200) {
        if (data.data?.payUrl) {
          const orderNo = data.data?.orderNo || ''
          if (orderNo) sessionStorage.setItem(purchaseOrderStorageKey, orderNo)
          ElMessage.success('支付订单已创建，正在跳转收银台')
          window.location.href = data.data.payUrl
          return
        }
        purchaseResult.value = data.data
        userBalance.value = Number(data.data.newBalance || 0)
        notifyBalanceRefresh()
        step.value = 4
        ElMessage.success('购买成功，授权已生成')
      } else {
        ElMessage.error(data.msg || '购买失败')
      }
    } catch {
      ElMessage.error('请求失败，请重试')
    } finally {
      purchasing.value = false
    }
  }
</script>

<style scoped lang="scss">
  .user-purchase {
    max-width: 960px;
    margin: 0 auto;
  }

  // 步骤条
  .steps-bar {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px 0 32px;
    gap: 0;

    .step {
      display: flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;

      .step-dot {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        background: var(--el-fill-color);
        color: var(--el-text-color-secondary);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 12px;
        font-weight: 700;
        transition: all 0.3s;
      }

      .step-text {
        font-size: 13px;
        color: var(--el-text-color-secondary);
        font-weight: 500;
        transition: color 0.3s;
      }

      &.active .step-dot {
        background: var(--el-color-primary);
        color: #fff;
        box-shadow: 0 2px 8px rgba(64, 158, 255, 0.3);
      }

      &.active .step-text {
        color: var(--el-color-primary);
      }

      &.done .step-dot {
        background: var(--el-color-success);
        color: #fff;
      }
    }

    .step-line {
      width: 60px;
      height: 2px;
      background: var(--el-fill-color);
      margin: 0 12px;
      border-radius: 1px;
      transition: background 0.3s;

      &.active {
        background: var(--el-color-primary);
      }
    }
  }

  // 过渡动画
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

  // Step 1 应用网格
  .app-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 14px;
  }

  .app-card {
    position: relative;
    padding: 20px 18px 16px;
    border-radius: 18px;
    border: 1.5px solid var(--el-border-color-lighter);
    cursor: pointer;
    transition: all 0.25s ease;
    background: linear-gradient(180deg, var(--el-bg-color) 0%, var(--el-bg-color-page) 100%);
    text-align: center;
    overflow: hidden;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      transform: translateY(-3px);
      box-shadow: 0 12px 30px rgba(15, 23, 42, 0.08);
    }

    &.active {
      border-color: var(--el-color-primary);
      background: linear-gradient(
        135deg,
        var(--el-color-primary-light-9) 0%,
        var(--el-bg-color) 100%
      );
      box-shadow: 0 10px 26px rgba(64, 158, 255, 0.16);
    }

    &.has-promo {
      border-color: var(--el-color-danger-light-5);
      background: linear-gradient(180deg, var(--el-color-danger-light-9) 0%, var(--el-bg-color) 70%);
    }
  }

  .app-promo-badge {
    position: absolute;
    top: 10px;
    right: 10px;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 3px 8px;
    border-radius: 999px;
    background: linear-gradient(135deg, #f43f5e, #e11d48);
    color: #fff;
    font-size: 10px;
    font-weight: 700;
    box-shadow: 0 4px 10px rgba(225, 29, 72, 0.22);
  }

  .app-icon-wrap {
    width: 50px;
    height: 50px;
    border-radius: 13px;
    background: linear-gradient(135deg, var(--el-color-primary-light-8), var(--el-color-primary-light-9));
    color: var(--el-color-primary);
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 10px;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.6);
  }

  .app-name {
    font-size: 14px;
    font-weight: 700;
    margin-bottom: 4px;
    color: var(--el-text-color-primary);
  }

  .app-desc {
    font-size: 11px;
    color: var(--el-text-color-secondary);
    margin-bottom: 12px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .app-footer {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    padding-top: 8px;
    border-top: 1px dashed var(--el-border-color-lighter);
  }

  .app-pricing {
    display: flex;
    align-items: baseline;
    gap: 3px;

    .price-currency {
      font-size: 12px;
      font-weight: 700;
      color: var(--el-color-primary);
    }
    .price-amount {
      font-size: 21px;
      font-weight: 800;
      color: var(--el-color-primary);
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;
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

  // Step 2 配置
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
    display: block;
    font-size: 13px;
    font-weight: 600;
    color: var(--el-text-color-primary);
    margin-bottom: 12px;
  }

  .type-options {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }

  .type-chip {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 16px;
    border-radius: 20px;
    border: 1.5px solid var(--el-border-color-lighter);
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    color: var(--el-text-color-regular);
    transition: all 0.2s;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      color: var(--el-color-primary);
    }

    &.active {
      border-color: var(--el-color-primary);
      background: var(--el-color-primary-light-9);
      color: var(--el-color-primary);
    }
  }

  .domain-input-wrap {
    position: relative;
    max-width: 380px;

    .input-icon {
      position: absolute;
      left: 14px;
      top: 50%;
      transform: translateY(-50%);
      color: var(--el-text-color-placeholder);
    }

    .domain-input {
      width: 100%;
      height: 40px;
      padding: 0 14px 0 40px;
      border-radius: 10px;
      border: 1.5px solid var(--el-border-color);
      outline: none;
      font-size: 14px;
      transition: all 0.2s;
      background: var(--el-bg-color);

      &:focus {
        border-color: var(--el-color-primary);
        box-shadow: 0 0 0 3px rgba(64, 158, 255, 0.1);
      }

      &::placeholder {
        color: var(--el-text-color-placeholder);
      }
    }
  }

  // 套餐卡片（第二步）
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
    border-radius: 16px;
    border: 1.5px solid var(--el-border-color-lighter);
    background: var(--el-bg-color);
    cursor: pointer;
    transition: all 0.22s ease;
    overflow: hidden;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      transform: translateY(-2px);
      box-shadow: 0 10px 26px rgba(15, 23, 42, 0.06);
    }

    &.active {
      border-color: var(--el-color-primary);
      background: linear-gradient(180deg, var(--el-color-primary-light-9) 0%, var(--el-bg-color) 100%);
      box-shadow: 0 8px 22px rgba(64, 158, 255, 0.14);
    }

    &.has-promo {
      border-color: var(--el-color-danger-light-5);
      background: linear-gradient(180deg, var(--el-color-danger-light-9) 0%, var(--el-bg-color) 70%);

      &.active {
        border-color: var(--el-color-danger);
        background: linear-gradient(180deg, var(--el-color-danger-light-8) 0%, var(--el-color-danger-light-9) 100%);
      }
    }
  }

  .promo-badge {
    position: absolute;
    top: 14px;
    right: 14px;
    padding: 4px 10px;
    border-radius: 999px;
    background: linear-gradient(135deg, #f43f5e, #e11d48);
    color: #fff;
    font-size: 11px;
    font-weight: 700;
    box-shadow: 0 6px 14px rgba(225, 29, 72, 0.22);
  }

  .plan-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    min-height: 22px;
  }

  .plan-name {
    font-size: 15px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  .plan-tag {
    padding: 4px 8px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 700;
    background: var(--el-color-primary-light-9);
    color: var(--el-color-primary);
  }

  .plan-tag.promo {
    background: var(--el-color-danger-light-8);
    color: var(--el-color-danger);
  }

  .plan-pricing {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }

  .plan-currency {
    font-size: 14px;
    font-weight: 700;
    color: var(--el-color-primary);
  }

  .plan-amount {
    font-size: 26px;
    font-weight: 800;
    color: var(--el-color-primary);
    font-family: 'DIN Alternate', 'Roboto Mono', monospace;
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

  // 预览卡
  .config-preview {
    width: 260px;
    flex-shrink: 0;
  }

  .preview-card {
    border-radius: 16px;
    border: 1px solid var(--el-border-color-lighter);
    overflow: hidden;
    background: var(--el-bg-color);
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);

    .preview-header {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 16px 20px;
      background: var(--el-color-primary-light-9);
      color: var(--el-color-primary);
      font-weight: 600;
      font-size: 14px;
    }

    .preview-body {
      padding: 16px 20px;

      .preview-item {
        display: flex;
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

          &.mono {
            font-family: 'Roboto Mono', monospace;
            font-size: 11px;
          }
        }
      }
    }

    .preview-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 14px 20px;
      border-top: 1px dashed var(--el-border-color-lighter);

      .preview-total-label {
        font-size: 13px;
        font-weight: 600;
      }

      .preview-total-price {
        font-size: 20px;
        font-weight: 800;
        color: var(--el-color-primary);
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      }
    }
  }

  // Step 3 支付
  .payment-layout {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .payment-detail-card,
  .payment-action-card {
    background: var(--el-bg-color);
    border-radius: 16px;
    border: 1px solid var(--el-border-color-lighter);
    padding: 24px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  }

  .payment-detail-card {
    flex: 1;

    .payment-header {
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 15px;
      font-weight: 600;
      margin-bottom: 20px;
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
    }

    .payment-total-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
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
          font-size: 28px;
          font-weight: 800;
          color: var(--el-color-primary);
          font-family: 'DIN Alternate', 'Roboto Mono', monospace;
        }
      }
    }
  }

  .payment-action-card {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 20px;

    .method-options {
      display: flex;
      gap: 10px;
    }

    .method-item {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      padding: 12px;
      border-radius: 10px;
      border: 1.5px solid var(--el-border-color-lighter);
      cursor: pointer;
      font-size: 13px;
      font-weight: 500;
      transition: all 0.2s;
      flex-direction: column;

      .method-balance {
        font-size: 11px;
        color: var(--el-color-success);
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      }

      .method-recharge-btn {
        margin-top: 4px;
        padding: 4px 10px;
        border: none;
        border-radius: 999px;
        color: var(--el-color-primary);
        background: var(--el-color-primary-light-9);
        cursor: pointer;
        font-size: 12px;
        font-weight: 700;
      }

      &.active {
        border-color: var(--el-color-primary);
        background: var(--el-color-primary-light-9);
      }
    }

    .balance-method {
      min-height: 116px;
    }
  }

  .purchase-btn {
    width: 100%;
    height: 48px;
    border: none;
    border-radius: 12px;
    background: linear-gradient(135deg, #409eff 0%, #2563eb 100%);
    color: #fff;
    font-size: 15px;
    font-weight: 700;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    box-shadow: 0 4px 14px rgba(64, 158, 255, 0.35);
    transition: all 0.25s;

    &:hover {
      transform: translateY(-1px);
      box-shadow: 0 6px 20px rgba(64, 158, 255, 0.45);
    }

    &:active {
      transform: translateY(0);
    }
  }

  .payment-hint {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    font-size: 11px;
    color: var(--el-text-color-placeholder);
  }

  .empty-card {
    padding: 40px 24px;
    border-radius: 16px;
    border: 1px dashed var(--el-border-color);
    color: var(--el-text-color-secondary);
    text-align: center;
    background: var(--el-bg-color);
  }

  .plan-count {
    margin-top: 2px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    font-weight: 600;
  }

  .section-title {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 16px;
    font-size: 16px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  .plan-grid {
    grid-template-columns: repeat(3, 1fr);
  }

  .domain-input:disabled {
    cursor: not-allowed;
    background: var(--el-fill-color-lighter);
  }

  .balance-warning {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    padding: 10px 12px;
    border-radius: 10px;
    background: var(--el-color-danger-light-9);
    color: var(--el-color-danger);
    font-size: 13px;
    text-align: center;

    button {
      border: none;
      border-radius: 999px;
      padding: 4px 10px;
      color: #fff;
      background: var(--el-color-danger);
      cursor: pointer;
      font-size: 12px;
      font-weight: 700;
    }
  }

  .purchase-btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
    transform: none;
    box-shadow: none;
  }

  .recharge-dialog-body {
    .recharge-amount-input {
      width: 100%;
    }

    .quick-amounts {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 8px;
      margin: 12px 0 18px;

      button {
        height: 34px;
        border: 1px solid var(--el-border-color-lighter);
        border-radius: 10px;
        background: var(--el-bg-color);
        color: var(--el-text-color-regular);
        cursor: pointer;
        font-weight: 700;
        transition: all 0.2s;

        &:hover {
          color: var(--el-color-primary);
          border-color: var(--el-color-primary);
          background: var(--el-color-primary-light-9);
        }
      }
    }

    .recharge-pay-label {
      margin-top: 6px;
    }

    .recharge-pay-types {
      width: 100%;
    }

    .recharge-pay-empty {
      margin: 0;
      padding: 10px 12px;
      border-radius: 10px;
      background: var(--el-color-warning-light-9);
      color: var(--el-color-warning);
      font-size: 13px;
    }

    .recharge-tip {
      margin: 12px 0 0;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }
  }

  .success-card {
    position: relative;
    max-width: 640px;
    margin: 0 auto;
    padding: 34px;
    border-radius: 24px;
    border: 1px solid rgba(22, 199, 154, 0.18);
    background:
      radial-gradient(circle at 18% 8%, rgba(22, 199, 154, 0.14), transparent 28%),
      linear-gradient(180deg, var(--el-bg-color) 0%, var(--el-bg-color-page) 100%);
    box-shadow: 0 18px 50px rgba(31, 45, 61, 0.08);
  }

  .success-hero {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 16px;
    margin-bottom: 24px;
    text-align: left;
  }

  .success-icon-wrap {
    width: 64px;
    height: 64px;
    border-radius: 22px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    background: linear-gradient(135deg, #20d6ad 0%, #12b886 100%);
    box-shadow: 0 12px 26px rgba(18, 184, 134, 0.28);
  }

  .success-title-block {
    h3 {
      margin: 0;
      font-size: 24px;
      font-weight: 800;
      color: var(--el-text-color-primary);
    }

    p {
      margin: 6px 0 0;
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }
  }

  .license-no-card {
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 18px 20px;
    border-radius: 16px;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.8);
    margin-bottom: 16px;
  }

  .license-no-label {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .license-no-value {
    font-size: 18px;
    font-weight: 800;
    letter-spacing: 0.4px;
    color: var(--el-color-primary);
    font-family: 'DIN Alternate', 'Roboto Mono', monospace;
    word-break: break-all;
  }

  .success-info-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
    margin-bottom: 16px;
  }

  .success-info-item {
    padding: 14px 16px;
    border-radius: 14px;
    background: rgba(255, 255, 255, 0.78);
    border: 1px solid var(--el-border-color-extra-light);

    .info-label {
      display: block;
      margin-bottom: 6px;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    .info-value {
      display: block;
      font-size: 15px;
      font-weight: 700;
      color: var(--el-text-color-primary);
    }
  }

  .success-amount-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 18px;
    border-radius: 16px;
    background: linear-gradient(135deg, var(--el-color-primary-light-9), var(--el-bg-color));
    border: 1px solid var(--el-color-primary-light-7);
    margin-bottom: 24px;

    span {
      font-size: 13px;
      color: var(--el-text-color-secondary);
    }

    strong {
      font-size: 24px;
      color: var(--el-color-primary);
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;
    }
  }

  .success-actions {
    display: flex;
    justify-content: center;

    .el-button {
      min-width: 140px;
      border-radius: 12px;
      font-weight: 700;
    }
  }

  // 底部导航
  .bottom-nav {
    display: flex;
    justify-content: center;
    gap: 12px;
    padding: 24px 0;
  }

  @media (max-width: 768px) {
    .app-grid {
      grid-template-columns: 1fr;
    }
    .config-layout {
      flex-direction: column;
    }
    .config-preview {
      width: 100%;
    }
    .payment-layout {
      flex-direction: column;
    }
    .payment-action-card {
      width: 100%;
    }
    .plan-grid {
      grid-template-columns: repeat(2, 1fr);
    }
    .success-card {
      padding: 24px 18px;
    }
    .success-hero {
      flex-direction: column;
      text-align: center;
    }
    .success-info-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
