<template>
  <div class="epay-config-page">
    <ElCard v-if="activeVersion === 'epay'" v-loading="loading" class="config-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <h2>支付配置</h2>
            <p>管理已启用支付插件的网关、商户信息、回调地址和默认支付方式</p>
          </div>
          <ElButton
            v-if="pluginEnabled"
            type="primary"
            :loading="saving"
            @click="handleSave"
          >
            保存配置
          </ElButton>
        </div>
      </template>

      <div v-if="!loading && !pluginEnabled" class="plugin-empty">
        <ElEmpty description="当前没有启用中的支付插件，请先到应用商店启用支付插件">
          <ElButton type="primary" @click="router.push('/plugin-store')">前往应用商店</ElButton>
        </ElEmpty>
      </div>
      <ElForm
        v-if="pluginEnabled"
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        class="config-form"
      >
        <ElRow :gutter="24">
          <ElCol :xs="24" :lg="15">
            <section class="section-card">
              <div class="section-title with-switch">
                <div>
                  <strong>商户配置</strong>
                  <span>商户 Key 留空表示保持已保存值，输入新值后会覆盖旧值。</span>
                </div>
                <div class="enable-switch">
                  <span>启用支付</span>
                  <ElSwitch
                    v-model="form.easypayEnabled"
                    inline-prompt
                    active-text="启用"
                    inactive-text="关闭"
                  />
                </div>
              </div>
              <ElRow :gutter="16">
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="易支付网关地址" prop="easypayGateway">
                    <ElInput
                      v-model.trim="form.easypayGateway"
                      placeholder="例如 https://pay.example.com"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="6">
                  <ElFormItem label="商户 PID" prop="easypayPid">
                    <ElInput v-model.trim="form.easypayPid" placeholder="商户 PID" />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="6">
                  <ElFormItem label="商户 Key" prop="easypayMerchantKey">
                    <ElInput
                      v-model.trim="form.easypayMerchantKey"
                      type="password"
                      show-password
                      :placeholder="form.easypayMerchantKeySet ? '已设置，留空不修改' : '商户 Key'"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="开启的支付方式" prop="easypayPayTypes">
                    <ElCheckboxGroup
                      v-model="form.easypayPayTypes"
                      @change="ensureDefaultTypeEnabled"
                    >
                      <ElCheckboxButton
                        v-for="item in paymentTypeOptions"
                        :key="item.value"
                        :value="item.value"
                      >
                        {{ item.label }}
                      </ElCheckboxButton>
                    </ElCheckboxGroup>
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="默认支付方式" prop="easypayDefaultType">
                    <div class="default-type-row">
                      <ElSelect
                        v-model="form.easypayDefaultType"
                        placeholder="请选择"
                        :disabled="enabledPaymentTypeOptions.length === 0"
                      >
                        <ElOption
                          v-for="item in enabledPaymentTypeOptions"
                          :key="item.value"
                          :label="item.label"
                          :value="item.value"
                        />
                      </ElSelect>
                      <ElButton type="primary" @click="openTestDialog">测试支付</ElButton>
                    </div>
                  </ElFormItem>
                </ElCol>
              </ElRow>
            </section>

            <section class="section-card">
              <div class="section-title">
                <div>
                  <strong>回调地址</strong>
                  <span>留空时系统会按当前请求域名自动生成默认地址。</span>
                </div>
              </div>
              <ElRow :gutter="16">
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="异步通知地址" prop="easypayNotifyUrl">
                    <ElInput
                      v-model.trim="form.easypayNotifyUrl"
                      placeholder="可选，留空自动生成"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="同步跳转地址" prop="easypayReturnUrl">
                    <ElInput
                      v-model.trim="form.easypayReturnUrl"
                      placeholder="可选，留空自动生成"
                    />
                  </ElFormItem>
                </ElCol>
              </ElRow>
            </section>
          </ElCol>

          <ElCol :xs="24" :lg="9">
            <div class="side-panel">
              <div class="side-title">配置状态</div>
              <ElDescriptions :column="1" border>
                <ElDescriptionsItem label="通道状态">
                  <ElTag :type="form.easypayEnabled ? 'success' : 'info'">
                    {{ form.easypayEnabled ? '已启用' : '已关闭' }}
                  </ElTag>
                </ElDescriptionsItem>
                <ElDescriptionsItem label="商户 PID">
                  {{ form.easypayPid || '-' }}
                </ElDescriptionsItem>
                <ElDescriptionsItem label="商户 Key">
                  <ElTag :type="keyReady ? 'success' : 'warning'">
                    {{ keyReady ? '已配置' : '未配置' }}
                  </ElTag>
                </ElDescriptionsItem>
                <ElDescriptionsItem label="默认方式">
                  {{ selectedPaymentTypeLabel }}
                </ElDescriptionsItem>
                <ElDescriptionsItem label="支付方式">
                  <div class="pay-type-tags">
                    <ElTag
                      v-for="item in paymentTypeOptions"
                      :key="item.value"
                      :type="form.easypayPayTypes.includes(item.value) ? 'success' : 'info'"
                      size="small"
                    >
                      {{ item.label
                      }}{{ form.easypayPayTypes.includes(item.value) ? '' : '（关）' }}
                    </ElTag>
                  </div>
                </ElDescriptionsItem>
              </ElDescriptions>
            </div>

            <div class="side-panel">
              <div class="side-title">默认回调路径</div>
              <div class="callback-list">
                <div class="callback-item">
                  <span>异步通知</span>
                  <code>/api/payment/easypay/notify</code>
                </div>
                <div class="callback-item">
                  <span>同步跳转</span>
                  <code>/api/payment/easypay/return</code>
                </div>
              </div>
              <ElAlert
                title="如果站点经过反向代理或网关，请优先填写公网可访问的完整 HTTPS 回调地址。"
                type="info"
                show-icon
                :closable="false"
              />
            </div>

            <div class="side-panel">
              <div class="side-title">保存规则</div>
              <ul class="rule-list">
                <li>启用通道前必须填写网关、PID 和商户 Key。</li>
                <li>商户 Key 不会明文回显，留空保存不会覆盖旧 Key。</li>
                <li>只有开启的支付方式会在用户支付时展示，至少保留一种。</li>
                <li>系统配置页不再保存或覆盖易支付配置。</li>
              </ul>
            </div>
          </ElCol>
        </ElRow>
      </ElForm>
    </ElCard>

    <ElCard v-if="pluginEnabled && activeVersion === 'epay-v2'" class="config-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <h2>易支付 V2 配置</h2>
            <p>管理易支付 V2 的网关、RSA 密钥、回调地址和默认支付方式</p>
          </div>
          <ElButton type="primary" :loading="savingV2" @click="handleSaveV2">保存配置</ElButton>
        </div>
      </template>

      <ElForm
        ref="formV2Ref"
        :model="formV2"
        :rules="rulesV2"
        label-position="top"
        class="config-form"
      >
        <ElRow :gutter="24">
          <ElCol :xs="24" :lg="15">
            <section class="section-card">
              <div class="section-title with-switch">
                <div>
                  <strong>商户配置</strong>
                  <span>RSA 密钥留空表示保持已保存值，输入新值后会覆盖旧值。</span>
                </div>
                <div class="enable-switch">
                  <span>启用支付</span>
                  <ElSwitch
                    v-model="formV2.easypayEnabled"
                    inline-prompt
                    active-text="启用"
                    inactive-text="关闭"
                  />
                </div>
              </div>
              <ElRow :gutter="16">
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="易支付 V2 网关地址" prop="easypayGateway">
                    <ElInput
                      v-model.trim="formV2.easypayGateway"
                      placeholder="例如 https://pay.example.com"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="商户 PID" prop="easypayPid">
                    <ElInput v-model.trim="formV2.easypayPid" placeholder="商户 PID" />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24">
                  <ElFormItem label="商户私钥（RSA）" prop="easypayMerchantKey">
                    <ElInput
                      v-model="formV2.easypayMerchantKey"
                      type="textarea"
                      :rows="4"
                      :placeholder="
                        formV2.easypayMerchantKeySet
                          ? '已设置，留空不修改'
                          : 'PEM 格式，支持 PKCS#1 / PKCS#8'
                      "
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24">
                  <ElFormItem label="平台公钥（RSA）" prop="easypayPlatformKey">
                    <ElInput
                      v-model="formV2.easypayPlatformKey"
                      type="textarea"
                      :rows="4"
                      :placeholder="
                        formV2.easypayPlatformKeySet
                          ? '已设置，留空不修改'
                          : 'PEM 格式，用于回调验签'
                      "
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="开启的支付方式" prop="easypayPayTypes">
                    <ElCheckboxGroup
                      v-model="formV2.easypayPayTypes"
                      @change="ensureDefaultTypeEnabledV2"
                    >
                      <ElCheckboxButton
                        v-for="item in paymentTypeOptions"
                        :key="item.value"
                        :value="item.value"
                      >
                        {{ item.label }}
                      </ElCheckboxButton>
                    </ElCheckboxGroup>
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="默认支付方式" prop="easypayDefaultType">
                    <div class="default-type-row">
                      <ElSelect
                        v-model="formV2.easypayDefaultType"
                        placeholder="请选择"
                        :disabled="enabledPaymentTypeOptionsV2.length === 0"
                      >
                        <ElOption
                          v-for="item in enabledPaymentTypeOptionsV2"
                          :key="item.value"
                          :label="item.label"
                          :value="item.value"
                        />
                      </ElSelect>
                      <ElButton type="primary" @click="openTestDialogV2">测试支付</ElButton>
                    </div>
                  </ElFormItem>
                </ElCol>
              </ElRow>
            </section>

            <section class="section-card">
              <div class="section-title">
                <div>
                  <strong>回调地址</strong>
                  <span>留空时系统会按当前请求域名自动生成默认地址。</span>
                </div>
              </div>
              <ElRow :gutter="16">
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="异步通知地址" prop="easypayNotifyUrl">
                    <ElInput
                      v-model.trim="formV2.easypayNotifyUrl"
                      placeholder="可选，留空自动生成"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="同步跳转地址" prop="easypayReturnUrl">
                    <ElInput
                      v-model.trim="formV2.easypayReturnUrl"
                      placeholder="可选，留空自动生成"
                    />
                  </ElFormItem>
                </ElCol>
              </ElRow>
            </section>
          </ElCol>

          <ElCol :xs="24" :lg="9">
            <div class="side-panel">
              <div class="side-title">配置状态</div>
              <ElDescriptions :column="1" border>
                <ElDescriptionsItem label="通道状态">
                  <ElTag :type="formV2.easypayEnabled ? 'success' : 'info'">
                    {{ formV2.easypayEnabled ? '已启用' : '已关闭' }}
                  </ElTag>
                </ElDescriptionsItem>
                <ElDescriptionsItem label="商户 PID">
                  {{ formV2.easypayPid || '-' }}
                </ElDescriptionsItem>
                <ElDescriptionsItem label="商户私钥">
                  <ElTag
                    :type="
                      formV2.easypayMerchantKeySet || formV2.easypayMerchantKey
                        ? 'success'
                        : 'warning'
                    "
                  >
                    {{
                      formV2.easypayMerchantKeySet || formV2.easypayMerchantKey
                        ? '已配置'
                        : '未配置'
                    }}
                  </ElTag>
                </ElDescriptionsItem>
                <ElDescriptionsItem label="平台公钥">
                  <ElTag
                    :type="
                      formV2.easypayPlatformKeySet || formV2.easypayPlatformKey
                        ? 'success'
                        : 'warning'
                    "
                  >
                    {{
                      formV2.easypayPlatformKeySet || formV2.easypayPlatformKey
                        ? '已配置'
                        : '未配置'
                    }}
                  </ElTag>
                </ElDescriptionsItem>
                <ElDescriptionsItem label="默认方式">
                  {{ selectedPaymentTypeLabelV2 }}
                </ElDescriptionsItem>
                <ElDescriptionsItem label="支付方式">
                  <div class="pay-type-tags">
                    <ElTag
                      v-for="item in paymentTypeOptions"
                      :key="item.value"
                      :type="formV2.easypayPayTypes.includes(item.value) ? 'success' : 'info'"
                      size="small"
                    >
                      {{ item.label
                      }}{{ formV2.easypayPayTypes.includes(item.value) ? '' : '（关）' }}
                    </ElTag>
                  </div>
                </ElDescriptionsItem>
              </ElDescriptions>
            </div>

            <div class="side-panel">
              <div class="side-title">默认回调路径</div>
              <div class="callback-list">
                <div class="callback-item">
                  <span>异步通知</span>
                  <code>/api/payment/easypay-v2/notify</code>
                </div>
                <div class="callback-item">
                  <span>同步跳转</span>
                  <code>/api/payment/easypay-v2/return</code>
                </div>
              </div>
              <ElAlert
                title="V2 使用 RSA-SHA256 签名，商户私钥用于下单签名，平台公钥用于回调验签。"
                type="info"
                show-icon
                :closable="false"
              />
            </div>

            <div class="side-panel">
              <div class="side-title">保存规则</div>
              <ul class="rule-list">
                <li>启用通道前必须填写网关、PID、商户私钥和平台公钥。</li>
                <li>RSA 密钥不会明文回显，留空保存不会覆盖旧密钥。</li>
                <li>只有开启的支付方式会在用户支付时展示，至少保留一种。</li>
                <li>V2 通过服务端下单返回收银台地址，与 V1 页面跳转不同。</li>
              </ul>
            </div>
          </ElCol>
        </ElRow>
      </ElForm>
    </ElCard>

    <ElDialog
      v-model="testDialog.visible"
      title="测试支付"
      width="440px"
      :close-on-click-modal="false"
      @closed="stopTestPolling"
    >
      <div v-if="testDialog.phase === 'form'" class="test-pay-body">
        <div class="test-field">
          <label>支付金额</label>
          <ElInputNumber
            v-model="testDialog.amount"
            :min="0.01"
            :max="100"
            :precision="2"
            :step="1"
            controls-position="right"
            class="test-amount-input"
          />
          <div class="quick-amounts">
            <button
              v-for="amount in quickTestAmounts"
              :key="amount"
              type="button"
              :class="{ active: testDialog.amount === amount }"
              @click="testDialog.amount = amount"
            >
              ¥{{ amount }}
            </button>
          </div>
        </div>
        <div class="test-field">
          <label>支付方式</label>
          <ElRadioGroup v-model="testDialog.payType" class="test-pay-types">
            <ElRadioButton
              v-for="item in enabledPaymentTypeOptions"
              :key="item.value"
              :value="item.value"
            >
              {{ item.label }}
            </ElRadioButton>
          </ElRadioGroup>
        </div>
        <div class="test-tip">
          <span>测试使用已保存的配置，修改后请先保存再测试。</span>
          <span>支付成功只记录结果，不会给任何账户入账。</span>
        </div>
      </div>

      <div v-else-if="testDialog.phase === 'waiting'" class="test-pay-body test-waiting">
        <div class="test-spinner"></div>
        <p class="test-waiting-title">等待支付结果</p>
        <p class="test-waiting-desc">已在新窗口打开收银台，完成支付后这里会自动显示结果</p>
        <div class="test-order-card">
          <span>测试订单号</span>
          <code>{{ testDialog.orderNo }}</code>
        </div>
      </div>

      <div v-else class="test-pay-body test-result">
        <div class="test-result-icon">
          <svg
            viewBox="0 0 24 24"
            width="26"
            height="26"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M4 12.5l5 5L20 6.5" />
          </svg>
        </div>
        <p class="test-result-title">支付成功，通道工作正常</p>
        <div class="test-result-list">
          <div class="result-row">
            <span>订单号</span>
            <code>{{ testDialog.result?.orderNo }}</code>
          </div>
          <div class="result-row">
            <span>支付方式</span>
            <em>{{ payTypeLabel(testDialog.result?.payType) }}</em>
          </div>
          <div class="result-row">
            <span>网关交易号</span>
            <code>{{ testDialog.result?.gatewayTradeNo || '-' }}</code>
          </div>
          <div class="result-row">
            <span>支付时间</span>
            <em>{{ testDialog.result?.paidAt || '-' }}</em>
          </div>
        </div>
        <div class="test-result-amount">
          <span>实付金额</span>
          <strong>¥{{ Number(testDialog.result?.amount || 0).toFixed(2) }}</strong>
        </div>
      </div>

      <template #footer>
        <template v-if="testDialog.phase === 'form'">
          <ElButton @click="testDialog.visible = false">取消</ElButton>
          <ElButton type="primary" :loading="testDialog.submitting" @click="submitPaymentTest">
            发起测试
          </ElButton>
        </template>
        <template v-else-if="testDialog.phase === 'waiting'">
          <ElButton @click="testDialog.visible = false">关闭</ElButton>
          <ElButton type="primary" @click="checkTestOrderNow">立即查询</ElButton>
        </template>
        <template v-else>
          <ElButton type="primary" @click="testDialog.visible = false">完成</ElButton>
        </template>
      </template>
    </ElDialog>

    <ElDialog
      v-model="testDialogV2.visible"
      title="测试支付（易支付 V2）"
      width="440px"
      :close-on-click-modal="false"
      @closed="stopTestPollingV2"
    >
      <div v-if="testDialogV2.phase === 'form'" class="test-pay-body">
        <div class="test-field">
          <label>支付金额</label>
          <ElInputNumber
            v-model="testDialogV2.amount"
            :min="0.01"
            :max="100"
            :precision="2"
            :step="1"
            controls-position="right"
            class="test-amount-input"
          />
          <div class="quick-amounts">
            <button
              v-for="amount in quickTestAmounts"
              :key="amount"
              type="button"
              :class="{ active: testDialogV2.amount === amount }"
              @click="testDialogV2.amount = amount"
            >
              ¥{{ amount }}
            </button>
          </div>
        </div>
        <div class="test-field">
          <label>支付方式</label>
          <ElRadioGroup v-model="testDialogV2.payType" class="test-pay-types">
            <ElRadioButton
              v-for="item in enabledPaymentTypeOptionsV2"
              :key="item.value"
              :value="item.value"
            >
              {{ item.label }}
            </ElRadioButton>
          </ElRadioGroup>
        </div>
        <div class="test-tip">
          <span>测试使用已保存的 V2 配置，修改后请先保存再测试。</span>
          <span>支付成功只记录结果，不会给任何账户入账。</span>
        </div>
      </div>

      <div v-else-if="testDialogV2.phase === 'waiting'" class="test-pay-body test-waiting">
        <div class="test-spinner"></div>
        <p class="test-waiting-title">等待支付结果</p>
        <p class="test-waiting-desc">已在新窗口打开收银台，完成支付后这里会自动显示结果</p>
        <div class="test-order-card">
          <span>测试订单号</span>
          <code>{{ testDialogV2.orderNo }}</code>
        </div>
      </div>

      <div v-else class="test-pay-body test-result">
        <div class="test-result-icon">
          <svg
            viewBox="0 0 24 24"
            width="26"
            height="26"
            fill="none"
            stroke="currentColor"
            stroke-width="3"
            stroke-linecap="round"
            stroke-linejoin="round"
          >
            <path d="M4 12.5l5 5L20 6.5" />
          </svg>
        </div>
        <p class="test-result-title">支付成功，通道工作正常</p>
        <div class="test-result-list">
          <div class="result-row">
            <span>订单号</span>
            <code>{{ testDialogV2.result?.orderNo }}</code>
          </div>
          <div class="result-row">
            <span>支付方式</span>
            <em>{{ payTypeLabel(testDialogV2.result?.payType) }}</em>
          </div>
          <div class="result-row">
            <span>网关交易号</span>
            <code>{{ testDialogV2.result?.gatewayTradeNo || '-' }}</code>
          </div>
          <div class="result-row">
            <span>支付时间</span>
            <em>{{ testDialogV2.result?.paidAt || '-' }}</em>
          </div>
        </div>
        <div class="test-result-amount">
          <span>实付金额</span>
          <strong>¥{{ Number(testDialogV2.result?.amount || 0).toFixed(2) }}</strong>
        </div>
      </div>

      <template #footer>
        <template v-if="testDialogV2.phase === 'form'">
          <ElButton @click="testDialogV2.visible = false">取消</ElButton>
          <ElButton type="primary" :loading="testDialogV2.submitting" @click="submitPaymentTestV2">
            发起测试
          </ElButton>
        </template>
        <template v-else-if="testDialogV2.phase === 'waiting'">
          <ElButton @click="testDialogV2.visible = false">关闭</ElButton>
          <ElButton type="primary" @click="checkTestOrderNowV2">立即查询</ElButton>
        </template>
        <template v-else>
          <ElButton type="primary" @click="testDialogV2.visible = false">完成</ElButton>
        </template>
      </template>
    </ElDialog>

    <ElBacktop target="#app-main" :right="32" :bottom="32" />
  </div>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import type {
    EpayPayType,
    PaymentConfigData,
    PaymentTestOrderData,
    PaymentV2ConfigData
  } from '@/api/system-manage'
  import {
    fetchCreatePaymentTest,
    fetchCreatePaymentV2Test,
    fetchPaymentConfig,
    fetchPaymentTestStatus,
    fetchPaymentV2Config,
    fetchPaymentV2TestStatus,
    fetchPluginList,
    fetchUpdatePaymentConfig,
    fetchUpdatePaymentV2Config
  } from '@/api/system-manage'

  defineOptions({ name: 'EpayConfig' })

  const route = useRoute()
  const router = useRouter()
  const formRef = ref<FormInstance>()
  const loading = ref(false)
  const saving = ref(false)
  const pluginEnabled = ref(true)
  const activeVersion = ref<'epay' | 'epay-v2'>('epay')
  const epayPluginOn = ref(false)
  const epayV2PluginOn = ref(false)

  const paymentTypeOptions: Array<{ label: string; value: EpayPayType }> = [
    { label: '支付宝', value: 'alipay' },
    { label: '微信支付', value: 'wxpay' },
    { label: 'QQ 钱包', value: 'qqpay' }
  ]
  const allPayTypes = paymentTypeOptions.map((item) => item.value)

  const form = reactive<PaymentConfigData>({
    easypayEnabled: false,
    easypayGateway: '',
    easypayPid: '',
    easypayMerchantKey: '',
    easypayMerchantKeySet: false,
    easypayDefaultType: 'alipay',
    easypayPayTypes: [...allPayTypes],
    easypayNotifyUrl: '',
    easypayReturnUrl: ''
  })

  const keyReady = computed(() => form.easypayMerchantKeySet || Boolean(form.easypayMerchantKey))
  const selectedPaymentTypeLabel = computed(() => {
    return paymentTypeOptions.find((item) => item.value === form.easypayDefaultType)?.label || '-'
  })
  const enabledPaymentTypeOptions = computed(() => {
    return paymentTypeOptions.filter((item) => form.easypayPayTypes.includes(item.value))
  })

  // 默认支付方式必须落在已开启的方式内，关掉当前默认方式时自动切换。
  const ensureDefaultTypeEnabled = () => {
    if (form.easypayPayTypes.length === 0) return
    if (!form.easypayPayTypes.includes(form.easypayDefaultType)) {
      form.easypayDefaultType = form.easypayPayTypes[0]
    }
  }

  const requiredWhenEnabled = (message: string) => {
    return (_rule: unknown, value: unknown, callback: (error?: Error) => void) => {
      if (form.easypayEnabled && !String(value || '').trim()) {
        callback(new Error(message))
        return
      }
      callback()
    }
  }

  const requireMerchantKey = (
    _rule: unknown,
    value: unknown,
    callback: (error?: Error) => void
  ) => {
    if (form.easypayEnabled && !form.easypayMerchantKeySet && !String(value || '').trim()) {
      callback(new Error('启用易支付前请输入商户 Key'))
      return
    }
    callback()
  }

  const validateOptionalHttpUrl = (label: string) => {
    return (_rule: unknown, value: unknown, callback: (error?: Error) => void) => {
      const url = String(value || '').trim()
      if (!url) {
        callback()
        return
      }

      try {
        const parsed = new URL(url)
        if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.host) {
          callback(new Error(`${label}仅支持 http 或 https`))
          return
        }
        callback()
      } catch {
        callback(new Error(`${label}格式不正确`))
      }
    }
  }

  const validatePayTypes = (_rule: unknown, value: unknown, callback: (error?: Error) => void) => {
    if (form.easypayEnabled && (!Array.isArray(value) || value.length === 0)) {
      callback(new Error('启用易支付前请至少开启一种支付方式'))
      return
    }
    callback()
  }

  const rules: FormRules<PaymentConfigData> = {
    easypayGateway: [
      { validator: requiredWhenEnabled('启用易支付前请输入网关地址'), trigger: 'blur' },
      { validator: validateOptionalHttpUrl('网关地址'), trigger: 'blur' }
    ],
    easypayPid: [{ validator: requiredWhenEnabled('启用易支付前请输入商户 PID'), trigger: 'blur' }],
    easypayMerchantKey: [{ validator: requireMerchantKey, trigger: 'blur' }],
    easypayPayTypes: [{ validator: validatePayTypes, trigger: 'change' }],
    easypayDefaultType: [{ required: true, message: '请选择默认支付方式', trigger: 'change' }],
    easypayNotifyUrl: [{ validator: validateOptionalHttpUrl('异步通知地址'), trigger: 'blur' }],
    easypayReturnUrl: [{ validator: validateOptionalHttpUrl('同步跳转地址'), trigger: 'blur' }]
  }

  onMounted(() => {
    loadConfig()
    handleTestReturn()
  })

  // 页面被 tab 缓存时，从其他页面切回来重新按当前启用插件刷新配置
  onActivated(() => {
    loadConfig()
  })

  onUnmounted(() => {
    stopTestPolling()
    stopTestPollingV2()
  })

  // ========== V2 表单 ==========
  const formV2Ref = ref<FormInstance>()
  const savingV2 = ref(false)

  const formV2 = reactive<PaymentV2ConfigData>({
    easypayEnabled: false,
    easypayGateway: '',
    easypayPid: '',
    easypayMerchantKey: '',
    easypayMerchantKeySet: false,
    easypayPlatformKey: '',
    easypayPlatformKeySet: false,
    easypayDefaultType: 'alipay',
    easypayPayTypes: [...allPayTypes],
    easypayNotifyUrl: '',
    easypayReturnUrl: ''
  })

  const enabledPaymentTypeOptionsV2 = computed(() => {
    return paymentTypeOptions.filter((item) => formV2.easypayPayTypes.includes(item.value))
  })
  const selectedPaymentTypeLabelV2 = computed(() => {
    return paymentTypeOptions.find((item) => item.value === formV2.easypayDefaultType)?.label || '-'
  })

  const ensureDefaultTypeEnabledV2 = () => {
    if (formV2.easypayPayTypes.length === 0) return
    if (!formV2.easypayPayTypes.includes(formV2.easypayDefaultType)) {
      formV2.easypayDefaultType = formV2.easypayPayTypes[0]
    }
  }

  const requireMerchantKeyV2 = (
    _rule: unknown,
    value: unknown,
    callback: (error?: Error) => void
  ) => {
    if (formV2.easypayEnabled && !formV2.easypayMerchantKeySet && !String(value || '').trim()) {
      callback(new Error('启用易支付 V2 前请输入商户私钥'))
      return
    }
    callback()
  }
  const requirePlatformKeyV2 = (
    _rule: unknown,
    value: unknown,
    callback: (error?: Error) => void
  ) => {
    if (formV2.easypayEnabled && !formV2.easypayPlatformKeySet && !String(value || '').trim()) {
      callback(new Error('启用易支付 V2 前请输入平台公钥'))
      return
    }
    callback()
  }

  const rulesV2: FormRules<PaymentV2ConfigData> = {
    easypayGateway: [
      { validator: requiredWhenEnabled('启用易支付 V2 前请输入网关地址'), trigger: 'blur' },
      { validator: validateOptionalHttpUrl('网关地址'), trigger: 'blur' }
    ],
    easypayPid: [
      { validator: requiredWhenEnabled('启用易支付 V2 前请输入商户 PID'), trigger: 'blur' }
    ],
    easypayMerchantKey: [{ validator: requireMerchantKeyV2, trigger: 'blur' }],
    easypayPlatformKey: [{ validator: requirePlatformKeyV2, trigger: 'blur' }],
    easypayPayTypes: [{ validator: validatePayTypes, trigger: 'change' }],
    easypayDefaultType: [{ required: true, message: '请选择默认支付方式', trigger: 'change' }],
    easypayNotifyUrl: [{ validator: validateOptionalHttpUrl('异步通知地址'), trigger: 'blur' }],
    easypayReturnUrl: [{ validator: validateOptionalHttpUrl('同步跳转地址'), trigger: 'blur' }]
  }

  const applyFormV2 = (data: PaymentV2ConfigData) => {
    const payTypes = Array.isArray(data.easypayPayTypes)
      ? allPayTypes.filter((item) => data.easypayPayTypes.includes(item))
      : [...allPayTypes]
    Object.assign(formV2, {
      easypayEnabled: data.easypayEnabled ?? false,
      easypayGateway: data.easypayGateway || '',
      easypayPid: data.easypayPid || '',
      easypayMerchantKey: '',
      easypayMerchantKeySet: data.easypayMerchantKeySet ?? false,
      easypayPlatformKey: '',
      easypayPlatformKeySet: data.easypayPlatformKeySet ?? false,
      easypayDefaultType: data.easypayDefaultType || 'alipay',
      easypayPayTypes: payTypes,
      easypayNotifyUrl: data.easypayNotifyUrl || '',
      easypayReturnUrl: data.easypayReturnUrl || ''
    })
    ensureDefaultTypeEnabledV2()
  }

  const handleSaveV2 = async () => {
    if (!formV2Ref.value) return
    await formV2Ref.value.validate()

    savingV2.value = true
    try {
      const data = await fetchUpdatePaymentV2Config({ ...formV2 })
      applyFormV2(data)
      ElMessage.success('易支付 V2 配置已更新')
    } finally {
      savingV2.value = false
    }
  }

  // ========== V2 测试支付 ==========
  const testDialogV2 = reactive({
    visible: false,
    submitting: false,
    phase: 'form' as 'form' | 'waiting' | 'success',
    amount: 0.01,
    payType: 'alipay' as EpayPayType,
    orderNo: '',
    result: null as PaymentTestOrderData | null
  })
  let testPollTimerV2: number | null = null

  const openTestDialogV2 = () => {
    stopTestPollingV2()
    testDialogV2.phase = 'form'
    testDialogV2.submitting = false
    testDialogV2.amount = 0.01
    testDialogV2.orderNo = ''
    testDialogV2.result = null
    const preferred = formV2.easypayPayTypes.includes(formV2.easypayDefaultType)
      ? formV2.easypayDefaultType
      : formV2.easypayPayTypes[0]
    testDialogV2.payType = preferred || 'alipay'
    testDialogV2.visible = true
  }

  const submitPaymentTestV2 = async () => {
    if (testDialogV2.submitting) return
    if (!testDialogV2.payType) {
      ElMessage.warning('请选择支付方式')
      return
    }

    testDialogV2.submitting = true
    try {
      const data = await fetchCreatePaymentV2Test({
        amount: Number(testDialogV2.amount),
        payType: testDialogV2.payType
      })
      if (!data.payUrl) {
        ElMessage.error('支付地址生成失败')
        return
      }
      testDialogV2.orderNo = data.orderNo
      testDialogV2.phase = 'waiting'
      window.open(data.payUrl, '_blank', 'noopener')
      startTestPollingV2()
    } finally {
      testDialogV2.submitting = false
    }
  }

  const startTestPollingV2 = () => {
    stopTestPollingV2()
    let attempts = 0
    const poll = async () => {
      if (!testDialogV2.visible || testDialogV2.phase !== 'waiting') return
      attempts += 1
      const paid = await queryTestOrderV2(false)
      if (paid) return
      if (attempts >= 60) {
        ElMessage.info('长时间未收到支付结果，可点击“立即查询”手动确认')
        return
      }
      testPollTimerV2 = window.setTimeout(poll, 3000)
    }
    testPollTimerV2 = window.setTimeout(poll, 3000)
  }

  const stopTestPollingV2 = () => {
    if (testPollTimerV2 !== null) {
      window.clearTimeout(testPollTimerV2)
      testPollTimerV2 = null
    }
  }

  const checkTestOrderNowV2 = async () => {
    const paid = await queryTestOrderV2(true)
    if (!paid) {
      ElMessage.info('还未查询到支付结果，请完成支付后再试')
    }
  }

  const queryTestOrderV2 = async (manual: boolean) => {
    if (!testDialogV2.orderNo) return false
    try {
      const data = await fetchPaymentV2TestStatus(testDialogV2.orderNo)
      if (data.status === 'paid') {
        stopTestPollingV2()
        testDialogV2.result = data
        testDialogV2.phase = 'success'
        return true
      }
    } catch {
      if (manual) {
        ElMessage.error('查询测试订单失败')
      }
    }
    return false
  }

  const testDialog = reactive({
    visible: false,
    submitting: false,
    phase: 'form' as 'form' | 'waiting' | 'success',
    amount: 0.01,
    payType: 'alipay' as EpayPayType,
    orderNo: '',
    result: null as PaymentTestOrderData | null
  })
  const quickTestAmounts = [0.01, 0.1, 1, 5]
  let testPollTimer: number | null = null

  const payTypeLabel = (value?: string) => {
    return paymentTypeOptions.find((item) => item.value === value)?.label || value || '-'
  }

  const openTestDialog = () => {
    stopTestPolling()
    testDialog.phase = 'form'
    testDialog.submitting = false
    testDialog.amount = 0.01
    testDialog.orderNo = ''
    testDialog.result = null
    const preferred = form.easypayPayTypes.includes(form.easypayDefaultType)
      ? form.easypayDefaultType
      : form.easypayPayTypes[0]
    testDialog.payType = preferred || 'alipay'
    testDialog.visible = true
  }

  const submitPaymentTest = async () => {
    if (testDialog.submitting) return
    if (!testDialog.payType) {
      ElMessage.warning('请选择支付方式')
      return
    }

    testDialog.submitting = true
    try {
      const data = await fetchCreatePaymentTest({
        amount: Number(testDialog.amount),
        payType: testDialog.payType
      })
      if (!data.payUrl) {
        ElMessage.error('支付地址生成失败')
        return
      }
      testDialog.orderNo = data.orderNo
      testDialog.phase = 'waiting'
      window.open(data.payUrl, '_blank', 'noopener')
      startTestPolling()
    } finally {
      testDialog.submitting = false
    }
  }

  const startTestPolling = () => {
    stopTestPolling()
    let attempts = 0
    const poll = async () => {
      if (!testDialog.visible || testDialog.phase !== 'waiting') return
      attempts += 1
      const paid = await queryTestOrder(false)
      if (paid) return
      if (attempts >= 60) {
        ElMessage.info('长时间未收到支付结果，可点击“立即查询”手动确认')
        return
      }
      testPollTimer = window.setTimeout(poll, 3000)
    }
    testPollTimer = window.setTimeout(poll, 3000)
  }

  const stopTestPolling = () => {
    if (testPollTimer !== null) {
      window.clearTimeout(testPollTimer)
      testPollTimer = null
    }
  }

  const checkTestOrderNow = async () => {
    const paid = await queryTestOrder(true)
    if (!paid) {
      ElMessage.info('还未查询到支付结果，请完成支付后再试')
    }
  }

  const queryTestOrder = async (manual: boolean) => {
    if (!testDialog.orderNo) return false
    try {
      const data = await fetchPaymentTestStatus(testDialog.orderNo)
      if (data.status === 'paid') {
        stopTestPolling()
        testDialog.result = data
        testDialog.phase = 'success'
        return true
      }
    } catch {
      if (manual) {
        ElMessage.error('查询测试订单失败')
      }
    }
    return false
  }

  // 从收银台同步跳转回来时（新标签页打开本页面），自动展示该笔测试的支付结果。
  const handleTestReturn = async () => {
    const orderNo = typeof route.query.rechargeOrder === 'string' ? route.query.rechargeOrder : ''
    if (!orderNo.startsWith('PT')) return

    const nextQuery = { ...route.query }
    delete nextQuery.rechargeOrder
    delete nextQuery.rechargeReturn
    router.replace({ path: route.path, query: nextQuery })

    try {
      const data = await fetchPaymentTestStatus(orderNo)
      if (data.status === 'paid') {
        testDialog.orderNo = orderNo
        testDialog.result = data
        testDialog.phase = 'success'
        testDialog.visible = true
      } else {
        ElMessage.info(`测试订单 ${orderNo} 尚未支付完成`)
      }
    } catch {
      ElMessage.error('查询测试订单失败')
    }
  }

  // 支付配置跟随支付插件启用状态动态加载：插件未启用时不展示具体配置。
  const loadConfig = async () => {
    loading.value = true
    try {
      const pluginData = await fetchPluginList()
      const plugins = (pluginData.categories || []).flatMap((group) => group.plugins)
      const epayPlugin = plugins.find((plugin) => plugin.id === 'epay')
      const epayV2Plugin = plugins.find((plugin) => plugin.id === 'epay-v2')
      epayPluginOn.value = Boolean(epayPlugin?.local && epayPlugin.enabled)
      epayV2PluginOn.value = Boolean(epayV2Plugin?.local && epayV2Plugin.enabled)
      pluginEnabled.value = epayPluginOn.value || epayV2PluginOn.value
      if (!pluginEnabled.value) return
      activeVersion.value = epayPluginOn.value ? 'epay' : 'epay-v2'
      if (epayPluginOn.value) {
        const data = await fetchPaymentConfig()
        applyForm(data)
      }
      if (epayV2PluginOn.value) {
        const dataV2 = await fetchPaymentV2Config()
        applyFormV2(dataV2)
      }
    } finally {
      loading.value = false
    }
  }

  const applyForm = (data: PaymentConfigData) => {
    const payTypes = Array.isArray(data.easypayPayTypes)
      ? allPayTypes.filter((item) => data.easypayPayTypes.includes(item))
      : [...allPayTypes]
    Object.assign(form, {
      easypayEnabled: data.easypayEnabled ?? false,
      easypayGateway: data.easypayGateway || '',
      easypayPid: data.easypayPid || '',
      easypayMerchantKey: '',
      easypayMerchantKeySet: data.easypayMerchantKeySet ?? false,
      easypayDefaultType: data.easypayDefaultType || 'alipay',
      easypayPayTypes: payTypes,
      easypayNotifyUrl: data.easypayNotifyUrl || '',
      easypayReturnUrl: data.easypayReturnUrl || ''
    })
    ensureDefaultTypeEnabled()
  }

  const handleSave = async () => {
    if (!formRef.value) return
    await formRef.value.validate()

    saving.value = true
    try {
      const data = await fetchUpdatePaymentConfig({ ...form })
      applyForm(data)
      ElMessage.success('易支付配置已更新')
    } finally {
      saving.value = false
    }
  }
</script>

<style scoped lang="scss">
  .epay-config-page {
    .config-card {
      min-height: 100%;
      border: 0;
    }

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;

      h2 {
        margin: 0;
        font-size: 20px;
        font-weight: 600;
        color: var(--art-gray-900);
      }

      p {
        margin: 6px 0 0;
        font-size: 13px;
        color: var(--art-gray-600);
      }
    }

    .plugin-empty {
      padding: 40px 0;
    }

    .config-form {
      :deep(.el-input),
      :deep(.el-select) {
        width: 100%;
      }

      .default-type-row {
        display: flex;
        gap: 12px;
        width: 100%;

        :deep(.el-select) {
          flex: 1;
          max-width: 200px;
        }

        .el-button {
          flex-shrink: 0;
        }
      }
    }

    .section-card,
    .side-panel {
      padding: 22px;
      margin-bottom: 18px;
      background: var(--art-main-bg-color);
      border: 1px solid var(--art-border-color);
      border-radius: 16px;
    }

    .section-title {
      margin-bottom: 20px;

      strong,
      span {
        display: block;
      }

      strong {
        font-size: 16px;
        color: var(--art-gray-900);
      }

      span {
        margin-top: 6px;
        font-size: 13px;
        color: var(--art-gray-600);
      }

      &.with-switch {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
      }
    }

    .enable-switch {
      display: flex;
      flex-shrink: 0;
      gap: 10px;
      align-items: center;

      span {
        margin: 0;
        font-size: 13px;
        font-weight: 600;
        color: var(--art-gray-800);
      }
    }

    .pay-type-tags {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
    }

    .side-panel {
      background: var(--art-gray-100);
    }

    .side-title {
      margin-bottom: 14px;
      font-size: 15px;
      font-weight: 600;
      color: var(--art-gray-800);
    }

    .callback-list {
      display: flex;
      flex-direction: column;
      gap: 12px;
      margin-bottom: 16px;
    }

    .callback-item {
      padding: 14px;
      background: var(--default-box-color);
      border-radius: 12px;

      span,
      code {
        display: block;
      }

      span {
        margin-bottom: 8px;
        font-size: 13px;
        color: var(--art-gray-600);
      }

      code {
        padding: 10px;
        font-size: 12px;
        color: var(--el-color-primary);
        word-break: break-all;
        background: var(--art-gray-100);
        border-radius: 8px;
      }
    }

    .rule-list {
      padding-left: 18px;
      margin: 0;
      color: var(--art-gray-700);

      li {
        margin-bottom: 10px;
        font-size: 13px;
        line-height: 1.7;
      }

      li:last-child {
        margin-bottom: 0;
      }
    }

    @media (max-width: 991px) {
      .card-header,
      .section-title.with-switch {
        align-items: flex-start;
        flex-direction: column;
      }
    }
  }

  .test-pay-body {
    .test-field {
      margin-bottom: 20px;

      label {
        display: block;
        margin-bottom: 10px;
        font-size: 13px;
        font-weight: 600;
        color: var(--art-gray-800);
      }
    }

    .test-amount-input {
      width: 100%;
    }

    .quick-amounts {
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 8px;
      margin-top: 10px;

      button {
        height: 32px;
        font-size: 13px;
        font-weight: 600;
        color: var(--art-gray-700);
        cursor: pointer;
        background: var(--art-main-bg-color);
        border: 1px solid var(--art-border-color);
        border-radius: 10px;
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
    }

    .test-pay-types {
      width: 100%;
    }

    .test-tip {
      padding: 12px 14px;
      background: var(--art-gray-100);
      border-radius: 12px;

      span {
        display: block;
        font-size: 12px;
        line-height: 1.8;
        color: var(--art-gray-600);
      }
    }

    &.test-waiting,
    &.test-result {
      text-align: center;
    }

    .test-spinner {
      width: 36px;
      height: 36px;
      margin: 10px auto 16px;
      border: 3px solid var(--el-color-primary-light-8);
      border-top-color: var(--el-color-primary);
      border-radius: 50%;
      animation: test-spin 0.9s linear infinite;
    }

    .test-waiting-title,
    .test-result-title {
      margin: 0 0 6px;
      font-size: 16px;
      font-weight: 600;
      color: var(--art-gray-900);
    }

    .test-waiting-desc {
      margin: 0 0 16px;
      font-size: 13px;
      color: var(--art-gray-600);
    }

    .test-order-card {
      padding: 14px;
      text-align: left;
      background: var(--art-gray-100);
      border-radius: 12px;

      span,
      code {
        display: block;
      }

      span {
        margin-bottom: 8px;
        font-size: 12px;
        color: var(--art-gray-600);
      }

      code {
        padding: 10px;
        font-size: 12px;
        color: var(--el-color-primary);
        word-break: break-all;
        background: var(--art-main-bg-color);
        border-radius: 8px;
      }
    }

    .test-result-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 48px;
      height: 48px;
      margin: 6px auto 14px;
      color: #fff;
      background: var(--el-color-success);
      border-radius: 16px;
      box-shadow: 0 8px 18px rgba(103, 194, 58, 0.3);
    }

    .test-result-title {
      margin-bottom: 18px;
    }

    .test-result-list {
      padding: 6px 14px;
      margin-bottom: 14px;
      text-align: left;
      background: var(--art-gray-100);
      border-radius: 12px;

      .result-row {
        display: flex;
        gap: 16px;
        align-items: center;
        justify-content: space-between;
        padding: 10px 0;
        border-bottom: 1px solid var(--art-border-color);

        &:last-child {
          border-bottom: 0;
        }

        span {
          flex-shrink: 0;
          font-size: 13px;
          color: var(--art-gray-600);
        }

        code,
        em {
          font-size: 13px;
          font-style: normal;
          font-weight: 500;
          color: var(--art-gray-900);
          word-break: break-all;
          text-align: right;
        }
      }
    }

    .test-result-amount {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 14px 16px;
      background: var(--el-color-primary-light-9);
      border: 1px solid var(--el-color-primary-light-7);
      border-radius: 12px;

      span {
        font-size: 13px;
        color: var(--art-gray-600);
      }

      strong {
        font-size: 20px;
        font-weight: 700;
        color: var(--el-color-primary);
      }
    }
  }

  @keyframes test-spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
