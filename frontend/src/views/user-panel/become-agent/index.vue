<template>
  <div class="upgrade-page">
    <section v-if="completed" class="result-card">
      <div class="result-icon">
        <iconify-icon icon="ri:user-star-fill" width="38" />
      </div>
      <p class="result-kicker">ACCOUNT UPGRADED</p>
      <h1>代理账户已开通</h1>
      <p class="result-description"> 原用户账户已经退出用户体系，授权和剩余余额已迁移至代理端。 </p>
      <div class="result-grid">
        <div>
          <span>代理等级</span>
          <strong>{{ result?.levelName }}</strong>
        </div>
        <div>
          <span>迁移余额</span>
          <strong>¥{{ money(result?.transferredBalance) }}</strong>
        </div>
        <div>
          <span>开通赠送</span>
          <strong>¥{{ money(result?.openingBonus) }}</strong>
        </div>
        <div>
          <span>代理账户余额</span>
          <strong>¥{{ money(result?.finalAgentBalance) }}</strong>
        </div>
        <div>
          <span>迁移授权</span>
          <strong>{{ result?.migratedLicenseCount || 0 }} 项</strong>
        </div>
      </div>
      <el-alert title="请使用原账号和原密码登录代理端" type="success" :closable="false" show-icon />
      <el-button type="primary" size="large" @click="goAgentLogin">
        前往代理商登录
        <iconify-icon icon="ri:arrow-right-line" />
      </el-button>
      <p class="redirect-tip">{{ redirectSeconds }} 秒后自动跳转</p>
    </section>

    <template v-else>
      <header class="page-hero">
        <div>
          <p class="eyebrow">BECOME AN AGENT</p>
          <h1>开通代理商</h1>
          <p>选择代理等级，支付开通费后，当前用户账户将立即转换为代理账户。</p>
        </div>
        <div class="balance-card">
          <span>当前用户余额</span>
          <strong>¥{{ money(balance) }}</strong>
          <small>余额支付后，剩余金额全部迁移至代理端</small>
        </div>
      </header>

      <el-alert
        class="risk-alert"
        title="账户升级不可撤销"
        description="升级成功后将无法继续登录用户端。原账号密码、实名资料、现有授权和剩余余额会迁移到新代理账户，历史支付与流水记录继续保留在原用户主体下。"
        type="warning"
        :closable="false"
        show-icon
      />

      <div v-loading="loading" class="upgrade-content">
        <section class="level-section">
          <div class="section-title">
            <div>
              <span>01</span>
              <div>
                <h2>选择代理等级</h2>
                <p>代理折扣代表开通授权时享受的结算折扣。</p>
              </div>
            </div>
            <el-tag v-if="levels.length" type="info" effect="plain">
              {{ levels.length }} 个可选等级
            </el-tag>
          </div>

          <el-empty v-if="!loading && levels.length === 0" description="暂未开放用户自助开通代理">
            <el-button @click="loadLevels">重新加载</el-button>
          </el-empty>

          <div v-else class="level-grid">
            <button
              v-for="level in levels"
              :key="level.id"
              type="button"
              class="level-card"
              :class="{ selected: selectedLevel?.id === level.id }"
              @click="selectedLevel = level"
            >
              <span class="level-check">
                <iconify-icon
                  :icon="
                    selectedLevel?.id === level.id
                      ? 'ri:checkbox-circle-fill'
                      : 'ri:checkbox-blank-circle-line'
                  "
                />
              </span>
              <div class="level-head">
                <span class="level-icon"><iconify-icon icon="ri:vip-crown-2-line" /></span>
                <div>
                  <h3>{{ level.name }}</h3>
                  <p>{{ level.code }}</p>
                </div>
              </div>
              <div class="level-price">
                <span>¥</span>
                <strong>{{ money(level.price) }}</strong>
                <small>一次性开通</small>
              </div>
              <div class="discount-row">
                <span>授权结算折扣</span>
                <strong>{{ level.discount }} 折</strong>
              </div>
              <div v-if="level.openingBonus > 0" class="discount-row bonus-row">
                <span>开通赠送余额</span>
                <strong>+ ¥{{ money(level.openingBonus) }}</strong>
              </div>
              <ul v-if="benefitLines(level).length">
                <li v-for="line in benefitLines(level)" :key="line">
                  <iconify-icon icon="ri:check-line" />
                  {{ line }}
                </li>
              </ul>
              <p v-else class="level-remark">
                {{ level.remark || '开通后可在代理端统一管理授权和账户资产。' }}
              </p>
            </button>
          </div>
        </section>

        <aside class="summary-card">
          <div class="summary-title">
            <span>02</span>
            <div>
              <h2>确认升级结果</h2>
              <p>金额以后端最终校验结果为准</p>
            </div>
          </div>

          <template v-if="selectedLevel">
            <div class="selected-level">
              <span><iconify-icon icon="ri:user-star-line" /></span>
              <div>
                <small>目标等级</small>
                <strong>{{ selectedLevel.name }}</strong>
              </div>
              <el-tag type="primary" effect="light">{{ selectedLevel.discount }} 折</el-tag>
            </div>

            <dl class="amount-list">
              <div>
                <dt>当前用户余额</dt>
                <dd>¥{{ money(balance) }}</dd>
              </div>
              <div>
                <dt>代理开通费</dt>
                <dd class="fee">- ¥{{ money(selectedLevel.price) }}</dd>
              </div>
              <div v-if="selectedLevel.openingBonus > 0">
                <dt>开通赠送余额</dt>
                <dd class="bonus">+ ¥{{ money(selectedLevel.openingBonus) }}</dd>
              </div>
              <div class="amount-total">
                <dt>预计代理账户余额</dt>
                <dd
                  >¥{{
                    money(
                      (payMethod === 'balance' ? selectedLevel.balanceAfterUpgrade : balance) +
                        selectedLevel.openingBonus
                    )
                  }}</dd
                >
              </div>
            </dl>

            <el-alert
              v-if="paymentIssue"
              class="payment-issue"
              :title="paymentIssue"
              type="error"
              :closable="false"
              show-icon
            />

            <div v-if="pendingOrder" class="pending-order">
              <el-alert
                title="已有待支付的代理开通订单"
                :description="`订单 ${pendingOrder.orderNo} 尚未完成，可继续前往收银台或取消后重新选择。`"
                type="warning"
                :closable="false"
                show-icon
              />
              <div class="pending-actions">
                <el-button type="primary" @click="continuePendingPayment">继续支付</el-button>
                <el-button :loading="cancelling" @click="cancelPendingPayment">取消订单</el-button>
              </div>
            </div>

            <template v-else-if="!paymentIssue">
              <div class="payment-section">
                <span class="payment-label">支付方式</span>
                <div class="payment-options">
                  <button
                    v-for="option in payOptions"
                    :key="option.code"
                    type="button"
                    class="payment-option"
                    :class="{ active: payMethod === option.code }"
                    @click="payMethod = option.code"
                  >
                    <iconify-icon :icon="option.icon" :style="{ color: option.color }" />
                    <span>{{ option.label }}</span>
                  </button>
                </div>
              </div>

              <el-alert
                v-if="payMethod === 'balance' && !selectedLevel.canAfford"
                title="当前余额不足，可选择已开启的在线支付方式"
                type="error"
                :closable="false"
                show-icon
              />

              <el-checkbox v-model="confirmed" class="confirm-checkbox">
                我已知晓升级不可撤销，且升级后需前往代理端登录
              </el-checkbox>

              <el-button
                type="primary"
                size="large"
                class="submit-button"
                :loading="submitting"
                :disabled="!confirmed || (payMethod === 'balance' && !selectedLevel.canAfford)"
                @click="submitUpgrade"
              >
                <iconify-icon icon="ri:secure-payment-line" />
                {{ submitButtonText }}
              </el-button>
            </template>
          </template>

          <div v-else class="summary-placeholder">
            <iconify-icon icon="ri:cursor-line" width="30" />
            <p>请先选择一个代理等级</p>
          </div>
        </aside>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import axios from 'axios'

  interface UpgradeLevel {
    id: number
    code: string
    name: string
    discount: number
    price: number
    openingBonus: number
    benefits: string
    remark: string
    canAfford: boolean
    balanceAfterUpgrade: number
    transferredBalance: number
    finalAgentBalance: number
  }

  interface UpgradeResult {
    orderNo: string
    agentId: number
    agentEmail: string
    levelName: string
    transferredBalance: number
    openingBonus: number
    finalAgentBalance: number
    migratedLicenseCount: number
    loginPath: string
  }

  interface PayOption {
    code: string
    channel?: string
    payType?: string
    label: string
    icon: string
    color: string
  }

  interface PendingUpgradeOrder {
    orderNo: string
    amount: number
    payChannel: string
    payMethod: string
    payUrl: string
  }

  const route = useRoute()
  const router = useRouter()
  const loading = ref(true)
  const submitting = ref(false)
  const balance = ref(0)
  const levels = ref<UpgradeLevel[]>([])
  const selectedLevel = ref<UpgradeLevel | null>(null)
  const payOptions = ref<PayOption[]>([
    { code: 'balance', label: '余额支付', icon: 'ri:wallet-3-line', color: '#2e7d32' }
  ])
  const payMethod = ref('balance')
  const pendingOrder = ref<PendingUpgradeOrder | null>(null)
  const paymentIssue = ref('')
  const cancelling = ref(false)
  const confirmed = ref(false)
  const completed = ref(false)
  const result = ref<UpgradeResult | null>(null)
  const redirectSeconds = ref(3)
  let redirectTimer: number | undefined
  const upgradeOrderStorageKey = 'user_panel_agent_upgrade_order'

  const selectedPayOption = computed(() => {
    return payOptions.value.find((option) => option.code === payMethod.value)
  })

  const submitButtonText = computed(() => {
    if (!selectedLevel.value) return '确认开通'
    if (submitting.value) {
      return payMethod.value === 'balance' ? '正在迁移账户资产' : '正在创建支付订单'
    }
    const label = selectedPayOption.value?.label || '在线支付'
    return `${label} ¥${money(selectedLevel.value.price)} 并开通`
  })

  function authHeaders() {
    return { Authorization: `Bearer ${localStorage.getItem('user_panel_token') || ''}` }
  }

  function money(value: unknown) {
    return Number(value || 0).toFixed(2)
  }

  function benefitLines(level: UpgradeLevel) {
    return (level.benefits || '')
      .split(/\r?\n|[,，]/)
      .map((line) => line.trim())
      .filter(Boolean)
      .slice(0, 5)
  }

  function clearPendingOrder() {
    pendingOrder.value = null
    localStorage.removeItem(upgradeOrderStorageKey)
  }

  async function restorePendingOrder() {
    paymentIssue.value = ''
    const raw = localStorage.getItem(upgradeOrderStorageKey)
    let stored: PendingUpgradeOrder | null = null
    if (raw) {
      try {
        stored = JSON.parse(raw) as PendingUpgradeOrder
      } catch {
        clearPendingOrder()
      }
    }
    if (stored && (!stored.orderNo || !stored.payUrl)) {
      clearPendingOrder()
      stored = null
    }
    const returnedOrder =
      typeof route.query.rechargeOrder === 'string' && route.query.rechargeOrder.startsWith('AU')
        ? route.query.rechargeOrder
        : ''
    const orderNo = returnedOrder || stored?.orderNo || ''
    if (!orderNo) return

    try {
      const { data } = await axios.get(`/api/user-panel/agent-upgrade/orders/${orderNo}`, {
        headers: authHeaders()
      })
      if (data.code !== 200) {
        paymentIssue.value = data.msg || '无法确认代理开通订单状态，请联系管理员核查'
        return
      }

      const status = data.data?.status
      if (status === 'pending') {
        if (stored?.orderNo === orderNo && stored.payUrl) {
          pendingOrder.value = stored
        } else {
          paymentIssue.value = '订单仍待支付，但当前页面缺少收银台地址，请取消订单后重新下单'
        }
        return
      }

      clearPendingOrder()
      if (status === 'paid' || status === 'processing') {
        paymentIssue.value =
          data.data?.errorMessage || '付款已确认，账户转换仍在处理中，请勿重复支付并联系管理员核查'
      } else if (status === 'failed') {
        paymentIssue.value = data.data?.errorMessage || '支付订单处理失败，请联系管理员核查'
      } else if (status === 'completed') {
        localStorage.removeItem('user_panel_token')
        localStorage.removeItem('user_panel_info')
        goAgentLogin()
      }
    } catch {
      paymentIssue.value = '无法确认代理开通订单状态，请稍后刷新或联系管理员核查'
    }
  }

  function continuePendingPayment() {
    if (pendingOrder.value?.payUrl) window.location.assign(pendingOrder.value.payUrl)
  }

  async function cancelPendingPayment() {
    if (!pendingOrder.value || cancelling.value) return
    cancelling.value = true
    try {
      const { data } = await axios.delete(
        `/api/user-panel/agent-upgrade/orders/${pendingOrder.value.orderNo}`,
        { headers: authHeaders() }
      )
      if (data.code !== 200) {
        ElMessage.error(data.msg || '取消支付订单失败')
        return
      }
      clearPendingOrder()
      paymentIssue.value = ''
      ElMessage.success('待支付订单已取消')
    } catch {
      ElMessage.error('取消支付订单失败')
    } finally {
      cancelling.value = false
    }
  }

  async function loadLevels() {
    loading.value = true
    try {
      const { data } = await axios.get('/api/user-panel/agent-upgrade/levels', {
        headers: authHeaders()
      })
      if (data.code !== 200) {
        ElMessage.error(data.msg || data.message || '加载代理等级失败')
        return
      }
      balance.value = Number(data.data?.balance || 0)
      levels.value = Array.isArray(data.data?.levels)
        ? data.data.levels.map((level: UpgradeLevel) => ({
            ...level,
            openingBonus: Number(level.openingBonus || 0),
            finalAgentBalance: Number(level.finalAgentBalance || 0)
          }))
        : []
      payOptions.value =
        Array.isArray(data.data?.payOptions) && data.data.payOptions.length > 0
          ? data.data.payOptions
          : [{ code: 'balance', label: '余额支付', icon: 'ri:wallet-3-line', color: '#2e7d32' }]
      if (!payOptions.value.some((option) => option.code === payMethod.value)) {
        payMethod.value = payOptions.value[0]?.code || 'balance'
      }
      const available = levels.value.find((level) => level.canAfford)
      selectedLevel.value = available || levels.value[0] || null
      await restorePendingOrder()
    } catch (error: any) {
      if (error?.response?.data?.data?.converted) {
        localStorage.removeItem('user_panel_token')
        localStorage.removeItem('user_panel_info')
        goAgentLogin()
        return
      }
      const message = error?.response?.data?.message || error?.response?.data?.msg
      ElMessage.error(message || '网络错误，请稍后重试')
    } finally {
      loading.value = false
    }
  }

  async function submitUpgrade() {
    if (!selectedLevel.value || !confirmed.value || submitting.value) return
    if (payMethod.value === 'balance' && !selectedLevel.value.canAfford) {
      ElMessage.warning('当前余额不足，请选择在线支付方式')
      return
    }
    const paymentLabel = selectedPayOption.value?.label || '所选方式'
    try {
      await ElMessageBox.confirm(
        `确认使用${paymentLabel}支付 ¥${money(selectedLevel.value.price)}，并将当前账户永久转换为“${selectedLevel.value.name}”吗？`,
        '最终确认',
        {
          confirmButtonText: '确认开通',
          cancelButtonText: '再想想',
          type: 'warning',
          distinguishCancelAndClose: true
        }
      )
    } catch {
      return
    }

    submitting.value = true
    try {
      const { data } = await axios.post(
        '/api/user-panel/agent-upgrade/orders',
        {
          levelId: selectedLevel.value.id,
          payMethod: payMethod.value,
          confirm: true
        },
        { headers: authHeaders() }
      )
      if (data.code !== 200) {
        ElMessage.error(data.msg || '代理账户开通失败')
        await loadLevels()
        return
      }

      if (data.data?.payUrl) {
        const order: PendingUpgradeOrder = {
          orderNo: data.data.orderNo,
          amount: Number(data.data.amount || selectedLevel.value.price),
          payChannel: data.data.payChannel || '',
          payMethod: data.data.payMethod || payMethod.value,
          payUrl: data.data.payUrl
        }
        pendingOrder.value = order
        localStorage.setItem(upgradeOrderStorageKey, JSON.stringify(order))
        window.location.assign(order.payUrl)
        return
      }

      clearPendingOrder()
      result.value = data.data as UpgradeResult
      completed.value = true
      localStorage.removeItem('user_panel_token')
      localStorage.removeItem('user_panel_info')
      redirectSeconds.value = 3
      redirectTimer = window.setInterval(() => {
        redirectSeconds.value -= 1
        if (redirectSeconds.value <= 0) goAgentLogin()
      }, 1000)
    } catch (error: any) {
      const message = error?.response?.data?.message || error?.response?.data?.msg
      ElMessage.error(message || '网络错误，账户资产未发生变化')
    } finally {
      submitting.value = false
    }
  }

  function goAgentLogin() {
    if (redirectTimer) window.clearInterval(redirectTimer)
    router.replace(result.value?.loginPath || '/agent-panel/login?upgraded=1')
  }

  onMounted(loadLevels)
  onBeforeUnmount(() => {
    if (redirectTimer) window.clearInterval(redirectTimer)
  })
</script>

<style scoped lang="scss">
  .upgrade-page {
    width: 100%;
    max-width: 1120px;
    margin: 0 auto;
    color: var(--el-text-color-primary);
  }

  .page-hero {
    display: flex;
    gap: 20px;
    align-items: center;
    justify-content: space-between;
    padding: 20px 22px;
    overflow: hidden;
    background:
      radial-gradient(circle at 82% 15%, rgb(64 158 255 / 14%), transparent 28%),
      linear-gradient(135deg, var(--el-bg-color) 0%, var(--el-color-primary-light-9) 100%);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 14px;

    h1 {
      margin: 3px 0 6px;
      font-size: 24px;
      line-height: 1.2;
    }

    p:not(.eyebrow) {
      max-width: 620px;
      margin: 0;
      font-size: 13px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
    }
  }

  .eyebrow,
  .result-kicker {
    margin: 0;
    font-size: 11px;
    font-weight: 700;
    color: var(--el-color-primary);
    letter-spacing: 1.4px;
  }

  .balance-card {
    display: flex;
    flex: 0 0 236px;
    flex-direction: column;
    justify-content: center;
    padding: 14px 18px;
    background: rgb(255 255 255 / 58%);
    backdrop-filter: blur(8px);
    border: 1px solid rgb(255 255 255 / 70%);
    border-radius: 12px;
    box-shadow: 0 8px 22px rgb(31 41 55 / 6%);

    span,
    small {
      color: var(--el-text-color-secondary);
    }

    span {
      font-size: 12px;
    }

    strong {
      margin: 2px 0 3px;
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      font-size: 24px;
      line-height: 1.2;
      color: var(--el-color-success);
    }

    small {
      font-size: 11px;
      line-height: 1.45;
    }
  }

  .risk-alert {
    margin-top: 12px;
  }

  .upgrade-content {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 320px;
    gap: 14px;
    min-height: 320px;
    margin-top: 14px;
  }

  .level-section,
  .summary-card,
  .result-card {
    padding: 18px;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 14px;
    box-shadow: 0 5px 18px rgb(15 23 42 / 4%);
  }

  .section-title,
  .summary-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 14px;

    > div {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    span:first-child {
      display: grid;
      place-items: center;
      width: 30px;
      height: 30px;
      font-size: 11px;
      font-weight: 700;
      color: var(--el-color-primary);
      background: var(--el-color-primary-light-9);
      border-radius: 8px;
    }

    h2 {
      margin: 0;
      font-size: 16px;
    }

    p {
      margin: 2px 0 0;
      font-size: 11px;
      color: var(--el-text-color-secondary);
    }
  }

  .level-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }

  .level-card {
    position: relative;
    min-width: 0;
    padding: 15px;
    color: inherit;
    text-align: left;
    cursor: pointer;
    background: var(--el-fill-color-blank);
    border: 1px solid var(--el-border-color);
    border-radius: 12px;
    transition:
      border-color 0.2s ease,
      box-shadow 0.2s ease,
      background-color 0.2s ease;

    &:hover {
      border-color: var(--el-color-primary-light-5);
      box-shadow: 0 5px 14px rgb(64 158 255 / 8%);
    }

    &.selected {
      background: var(--el-color-primary-light-9);
      border-color: var(--el-color-primary);
      box-shadow: 0 6px 16px rgb(64 158 255 / 10%);
    }

    .level-check {
      position: absolute;
      top: 12px;
      right: 12px;
      font-size: 18px;
      color: var(--el-color-primary);
    }
  }

  .level-head {
    display: flex;
    gap: 10px;
    align-items: center;
    padding-right: 24px;

    .level-icon {
      display: grid;
      flex: 0 0 auto;
      place-items: center;
      width: 36px;
      height: 36px;
      font-size: 18px;
      color: #d99a1b;
      background: #fff6dd;
      border-radius: 9px;
    }

    h3,
    p {
      margin: 0;
    }

    h3 {
      overflow: hidden;
      font-size: 15px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    p {
      margin-top: 1px;
      overflow: hidden;
      font-size: 10px;
      color: var(--el-text-color-placeholder);
      text-overflow: ellipsis;
      text-transform: uppercase;
      white-space: nowrap;
    }
  }

  .level-price {
    display: flex;
    align-items: baseline;
    margin: 14px 0 10px;
    color: var(--el-color-primary);

    strong {
      margin: 0 4px 0 2px;
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      font-size: 24px;
      line-height: 1.1;
    }

    small {
      font-size: 11px;
      color: var(--el-text-color-secondary);
    }
  }

  .discount-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
    font-size: 12px;
    border-top: 1px dashed var(--el-border-color-light);
    border-bottom: 1px dashed var(--el-border-color-light);

    span {
      color: var(--el-text-color-secondary);
    }

    &.bonus-row {
      color: var(--el-color-success);
      border-top: 0;
    }
  }

  .level-card ul {
    display: grid;
    gap: 5px;
    padding: 0;
    margin: 10px 0 0;
    font-size: 11px;
    line-height: 1.45;
    list-style: none;

    li {
      display: flex;
      gap: 5px;
      align-items: flex-start;
    }

    svg {
      flex: 0 0 auto;
      margin-top: 2px;
      color: var(--el-color-success);
    }
  }

  .level-remark {
    min-height: 30px;
    margin: 10px 0 0;
    font-size: 11px;
    line-height: 1.5;
    color: var(--el-text-color-secondary);
  }

  .summary-card {
    position: sticky;
    top: 16px;
    align-self: start;
  }

  .selected-level {
    display: flex;
    gap: 9px;
    align-items: center;
    padding: 11px;
    background: var(--el-fill-color-light);
    border-radius: 10px;

    > span {
      display: grid;
      flex: 0 0 auto;
      place-items: center;
      width: 32px;
      height: 32px;
      font-size: 17px;
      color: var(--el-color-primary);
      background: var(--el-bg-color);
      border-radius: 8px;
    }

    div {
      display: flex;
      min-width: 0;
      flex: 1;
      flex-direction: column;
    }

    small {
      font-size: 11px;
      color: var(--el-text-color-secondary);
    }

    strong {
      overflow: hidden;
      font-size: 14px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .amount-list {
    margin: 14px 0;

    > div {
      display: flex;
      gap: 12px;
      align-items: center;
      justify-content: space-between;
      padding: 7px 0;
      font-size: 12px;
    }

    dt {
      color: var(--el-text-color-secondary);
    }

    dd {
      margin: 0;
      font-weight: 600;
      text-align: right;
    }

    .fee {
      color: var(--el-color-danger);
    }

    .bonus {
      color: var(--el-color-success);
    }

    .amount-total {
      padding-top: 11px;
      margin-top: 4px;
      border-top: 1px solid var(--el-border-color-lighter);

      dd {
        font-family: 'DIN Alternate', 'Roboto Mono', monospace;
        font-size: 17px;
        color: var(--el-color-success);
      }
    }
  }

  .payment-issue {
    margin-bottom: 10px;
  }

  .payment-section {
    margin-top: 13px;
  }

  .payment-label {
    display: block;
    margin-bottom: 7px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .payment-options {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 7px;
  }

  .payment-option {
    display: flex;
    min-width: 0;
    gap: 6px;
    align-items: center;
    justify-content: center;
    padding: 8px 9px;
    overflow: hidden;
    font-size: 12px;
    color: var(--el-text-color-primary);
    cursor: pointer;
    background: var(--el-fill-color-blank);
    border: 1px solid var(--el-border-color);
    border-radius: 8px;

    svg {
      flex: 0 0 auto;
      font-size: 16px;
    }

    span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    &.active {
      color: var(--el-color-primary);
      background: var(--el-color-primary-light-9);
      border-color: var(--el-color-primary);
    }
  }

  .pending-order {
    display: grid;
    gap: 10px;
  }

  .pending-actions {
    display: flex;

    .el-button {
      flex: 1;
    }
  }

  .confirm-checkbox {
    height: auto;
    margin-top: 13px;
    white-space: normal;

    :deep(.el-checkbox__label) {
      font-size: 12px;
      line-height: 1.45;
      white-space: normal;
    }
  }

  .submit-button {
    width: 100%;
    margin-top: 13px;
  }

  .summary-placeholder {
    display: grid;
    place-content: center;
    min-height: 160px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
    text-align: center;

    svg {
      margin: 0 auto;
    }
  }

  .result-card {
    max-width: 620px;
    padding: 32px;
    margin: 24px auto;
    text-align: center;

    h1 {
      margin: 6px 0;
      font-size: 26px;
    }

    .el-alert {
      margin: 18px 0;
      text-align: left;
    }
  }

  .result-icon {
    display: grid;
    place-items: center;
    width: 60px;
    height: 60px;
    margin: 0 auto 14px;
    color: #fff;
    background: linear-gradient(135deg, var(--el-color-success), #20b985);
    border-radius: 18px;
    box-shadow: 0 10px 22px rgb(32 185 133 / 22%);
  }

  .result-description,
  .redirect-tip {
    color: var(--el-text-color-secondary);
  }

  .result-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
    gap: 8px;
    margin: 18px 0;

    div {
      display: flex;
      flex-direction: column;
      padding: 11px 8px;
      background: var(--el-fill-color-light);
      border-radius: 10px;
    }

    span {
      margin-bottom: 3px;
      font-size: 11px;
      color: var(--el-text-color-secondary);
    }
  }

  .redirect-tip {
    margin: 12px 0 0;
    font-size: 12px;
  }

  @media (width <= 980px) {
    .upgrade-content {
      grid-template-columns: 1fr;
    }

    .summary-card {
      position: static;
      width: auto;
    }
  }

  @media (width <= 680px) {
    .page-hero {
      align-items: stretch;
      flex-direction: column;
      gap: 12px;
      padding: 16px;
    }

    .balance-card {
      flex-basis: auto;
      padding: 12px 14px;
    }

    .upgrade-content {
      gap: 10px;
      margin-top: 10px;
    }

    .level-grid {
      grid-template-columns: 1fr;
    }

    .section-title,
    .summary-title {
      gap: 10px;
      align-items: flex-start;
    }

    .level-section,
    .summary-card,
    .result-card {
      padding: 14px;
    }

    .result-card {
      margin: 12px auto;
    }
  }

  @media (width <= 420px) {
    .result-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
