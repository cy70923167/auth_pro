<template>
  <div class="panel-profile">
    <!-- 顶部用户卡片 -->
    <div class="art-card p-6 mb-5 profile-hero">
      <div class="hero-left">
        <el-avatar :size="72" class="hero-avatar">
          <iconify-icon icon="ri:user-3-fill" width="36" />
        </el-avatar>
        <div class="hero-info">
          <h3 class="hero-name">{{ profile.name || '-' }}</h3>
          <div class="hero-meta">
            <el-tag type="warning" size="small" effect="dark">{{
              profile.levelName || '-'
            }}</el-tag>
            <el-tag v-if="profile.realnameVerified" type="success" size="small">
              <iconify-icon icon="ri:shield-check-fill" width="12" />
              已实名
            </el-tag>
            <el-tag v-else-if="profile.realnameEnabled" type="warning" size="small">未实名</el-tag>
            <span class="hero-id">ID: AG-{{ String(profile.id).padStart(5, '0') }}</span>
          </div>
          <p class="hero-desc"
            >注册于 {{ profile.createdAt || '-' }} · 累计开通 {{ profile.licenseCount }} 个授权</p
          >
        </div>
      </div>
      <div class="hero-stats">
        <div class="hero-stat-item">
          <span class="stat-num">¥{{ balanceText }}</span>
          <span class="stat-label">账户余额</span>
        </div>
        <div class="hero-stat-item">
          <span class="stat-num">{{ profile.discount || '-' }}</span>
          <span class="stat-label">专属折扣</span>
        </div>
        <div class="hero-stat-item">
          <span class="stat-num">{{ profile.quotaRemain }}</span>
          <span class="stat-label">剩余配额</span>
        </div>
      </div>
    </div>

    <ElRow :gutter="20">
      <!-- 个人信息 -->
      <ElCol :sm="24" :lg="12">
        <div class="art-card p-5 mb-5">
          <div class="section-header">
            <iconify-icon icon="ri:user-settings-line" width="20" class="section-icon" />
            <h4>基本信息</h4>
          </div>
          <el-form
            :model="profileForm"
            :rules="profileRules"
            ref="profileRef"
            label-width="90px"
            class="mt-4"
          >
            <el-form-item label="代理商名称" prop="name">
              <el-input v-model="profileForm.name" maxlength="50" />
            </el-form-item>
            <el-form-item label="联系方式" prop="contact">
              <el-input
                v-model="profileForm.contact"
                placeholder="手机号 / 微信 / QQ"
                maxlength="100"
              />
            </el-form-item>
            <el-form-item label="登录账号">
              <el-input :model-value="profile.email" disabled />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="saving" @click="handleSaveProfile">
                <iconify-icon icon="ri:save-line" width="16" class="mr-1" />
                保存修改
              </el-button>
            </el-form-item>
          </el-form>
        </div>
      </ElCol>

      <!-- 修改密码 -->
      <ElCol :sm="24" :lg="12">
        <div class="art-card p-5 mb-5">
          <div class="section-header">
            <iconify-icon icon="ri:lock-password-line" width="20" class="section-icon" />
            <h4>安全设置</h4>
          </div>
          <el-form
            :model="passwordForm"
            :rules="passwordRules"
            ref="passwordRef"
            label-width="90px"
            class="mt-4"
          >
            <el-form-item label="当前密码" prop="oldPassword">
              <el-input
                v-model="passwordForm.oldPassword"
                type="password"
                show-password
                placeholder="请输入当前密码"
              />
            </el-form-item>
            <el-form-item label="新密码" prop="newPassword">
              <el-input
                v-model="passwordForm.newPassword"
                type="password"
                show-password
                placeholder="6-20位字符"
              />
            </el-form-item>
            <el-form-item label="确认密码" prop="confirmPassword">
              <el-input
                v-model="passwordForm.confirmPassword"
                type="password"
                show-password
                placeholder="再次输入新密码"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="changingPwd" @click="handleChangePassword">
                <iconify-icon icon="ri:shield-check-line" width="16" class="mr-1" />
                修改密码
              </el-button>
            </el-form-item>
          </el-form>

          <el-divider />

          <div class="security-tips">
            <iconify-icon icon="ri:information-line" width="16" color="#909399" />
            <span>为保障账户安全，建议定期更换密码并使用包含字母、数字的组合</span>
          </div>
        </div>
      </ElCol>
    </ElRow>

    <div class="art-card p-5 realname-card">
      <div class="section-header">
        <iconify-icon icon="ri:verified-badge-line" width="20" class="section-icon" />
        <h4>实名认证</h4>
      </div>

      <div v-if="profile.realnameVerified" class="realname-complete">
        <div class="verified-title">
          <iconify-icon icon="ri:shield-check-fill" width="20" />
          <span>已完成实名认证</span>
        </div>
        <div class="realname-details">
          <div
            ><span>姓名</span><strong>{{ profile.realName }}</strong></div
          >
          <div
            ><span>身份证号</span><strong>{{ profile.realIdCard }}</strong></div
          >
          <div
            ><span>认证时间</span><strong>{{ profile.realnameAt || '-' }}</strong></div
          >
        </div>
      </div>

      <div v-else class="realname-content">
        <el-alert
          v-if="!profile.realnameEnabled"
          type="info"
          :closable="false"
          title="当前站点未开启实名认证"
        />
        <template v-else>
          <el-alert
            type="warning"
            :closable="false"
            title="部分应用要求代理商完成实名认证后才能安装使用"
            class="mb-4"
          />
          <el-form :model="realnameForm" label-width="90px" class="realname-form">
            <el-form-item label="真实姓名">
              <el-input
                v-model="realnameForm.realName"
                maxlength="30"
                placeholder="请输入与身份证一致的姓名"
              />
            </el-form-item>
            <el-form-item label="身份证号">
              <el-input
                v-model="realnameForm.idCard"
                maxlength="18"
                placeholder="请输入18位身份证号"
              />
            </el-form-item>
            <el-form-item v-if="needRealnameMobile" label="手机号">
              <el-input
                v-model="realnameForm.mobile"
                maxlength="11"
                placeholder="请输入本人实名手机号"
              />
              <div class="realname-mobile-tip">仅用于本次三要素核验，不会保存到代理资料</div>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="realnameSubmitting" @click="handleRealnameSubmit">
                <iconify-icon icon="ri:qr-scan-2-line" width="16" class="mr-1" />
                {{ realnameSubmitButtonText }}
              </el-button>
            </el-form-item>
          </el-form>
        </template>
      </div>
    </div>

    <el-dialog
      v-model="certifyDialogVisible"
      :title="certifyDialogTitle"
      width="360px"
      :close-on-click-modal="false"
      @close="stopCertifyPolling"
    >
      <div class="certify-dialog">
        <img v-if="certifyQrDataUrl" :src="certifyQrDataUrl" alt="认证二维码" class="certify-qr" />
        <p class="certify-tip">
          {{ certifyDialogTip }}
        </p>
        <el-button
          v-if="certifyProvider !== 'kuaitong' && certifyProvider !== 'tencent'"
          type="primary"
          link
          @click="openCertifyUrl"
        >
          在浏览器中打开认证页面
        </el-button>
        <div class="certify-status">
          <el-icon v-if="certifyPolling" class="is-loading"><Loading /></el-icon>
          <span>{{ certifyStatusText }}</span>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, onBeforeUnmount, reactive, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import { Loading } from '@element-plus/icons-vue'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import QRCode from 'qrcode'
  import axios from 'axios'

  function getToken() {
    return localStorage.getItem('agent_panel_token') || ''
  }
  const headers = () => ({ Authorization: `Bearer ${getToken()}` })

  const profileRef = ref()
  const passwordRef = ref()
  const saving = ref(false)
  const changingPwd = ref(false)

  const profile = reactive({
    id: 0,
    email: '',
    name: '',
    contact: '',
    level: '',
    levelName: '',
    discount: '',
    balance: 0,
    quotaRemain: 0,
    licenseCount: 0,
    createdAt: '',
    realnameEnabled: false,
    realnameProvider: '',
    realnameAuthMode: '',
    realnameVerified: false,
    realName: '',
    realIdCard: '',
    realnameAt: ''
  })

  const balanceText = computed(() =>
    Number(profile.balance || 0).toLocaleString('zh-CN', {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2
    })
  )

  const profileForm = reactive({ name: '', contact: '' })

  const profileRules = {
    name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
  }

  const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
  const realnameForm = reactive({ realName: '', idCard: '', mobile: '' })
  const realnameSubmitting = ref(false)
  const certifyDialogVisible = ref(false)
  const certifyQrDataUrl = ref('')
  const certifyUrl = ref('')
  const certifyOuterOrderNo = ref('')
  const certifyId = ref('')
  const certifyProvider = ref('')
  const faceToken = ref('')
  const certifyPolling = ref(false)
  const certifyStatusText = ref('等待认证...')
  let certifyTimer: ReturnType<typeof setInterval> | null = null

  const validateConfirm = (_rule: any, value: string, callback: any) => {
    if (value !== passwordForm.newPassword) {
      callback(new Error('两次输入的密码不一致'))
    } else {
      callback()
    }
  }

  const passwordRules = {
    oldPassword: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
    newPassword: [
      { required: true, message: '请输入新密码', trigger: 'blur' },
      { min: 6, max: 20, message: '密码长度 6-20 位', trigger: 'blur' }
    ],
    confirmPassword: [
      { required: true, message: '请再次输入新密码', trigger: 'blur' },
      { validator: validateConfirm, trigger: 'blur' }
    ]
  }

  async function fetchProfile() {
    try {
      const { data } = await axios.get('/api/agent-panel/profile', { headers: headers() })
      if (data.code === 200 && data.data) {
        Object.assign(profile, data.data)
        profileForm.name = data.data.name || ''
        profileForm.contact = data.data.contact || ''
      }
    } catch {
      // 资料加载失败时保留空态，由用户刷新页面重试。
    }
  }

  function handleSaveProfile() {
    profileRef.value?.validate(async (valid: boolean) => {
      if (!valid) return
      saving.value = true
      try {
        const { data } = await axios.put(
          '/api/agent-panel/profile',
          { name: profileForm.name, contact: profileForm.contact },
          { headers: headers() }
        )
        if (data.code === 200) {
          ElMessage.success('个人信息已保存')
          fetchProfile()
        } else {
          ElMessage.error(data.msg || '保存失败')
        }
      } catch {
        ElMessage.error('保存失败')
      } finally {
        saving.value = false
      }
    })
  }

  function handleChangePassword() {
    passwordRef.value?.validate(async (valid: boolean) => {
      if (!valid) return
      changingPwd.value = true
      try {
        const { data } = await axios.post(
          '/api/agent-panel/change-password',
          { oldPassword: passwordForm.oldPassword, newPassword: passwordForm.newPassword },
          { headers: headers() }
        )
        if (data.code === 200) {
          ElMessage.success('密码修改成功，下次登录请使用新密码')
          passwordForm.oldPassword = ''
          passwordForm.newPassword = ''
          passwordForm.confirmPassword = ''
          passwordRef.value?.clearValidate()
        } else {
          ElMessage.error(data.msg || '修改失败')
        }
      } catch {
        ElMessage.error('修改失败')
      } finally {
        changingPwd.value = false
      }
    })
  }

  async function handleRealnameSubmit() {
    const realName = realnameForm.realName.trim()
    const idCard = realnameForm.idCard.trim().toUpperCase()
    const mobile = realnameForm.mobile.trim()
    if (realName.length < 2) {
      ElMessage.warning('请输入真实姓名')
      return
    }
    if (!/^\d{17}[\dX]$/.test(idCard)) {
      ElMessage.warning('请输入正确的18位身份证号')
      return
    }
    if (needRealnameMobile.value && !/^1[3-9]\d{9}$/.test(mobile)) {
      ElMessage.warning('请输入正确的11位手机号')
      return
    }

    realnameSubmitting.value = true
    try {
      const { data } = await axios.post(
        '/api/agent-panel/realname/init',
        needRealnameMobile.value ? { realName, idCard, mobile } : { realName, idCard },
        { headers: headers() }
      )
      if (data.code !== 200) {
        ElMessage.error(data.msg || '实名认证失败')
        return
      }
      if (data.data?.status === 'passed') {
        ElMessage.success('实名认证成功')
        resetRealnameForm()
        await fetchProfile()
        return
      }
      certifyProvider.value = data.data?.provider || profile.realnameProvider
      // 小沐聚合实名：跳转上游认证页，认证单用 faceToken 轮询
      if (certifyProvider.value === 'xiaomu' && data.data?.faceToken) {
        faceToken.value = data.data.faceToken
        certifyUrl.value = data.data.certifyUrl || ''
        certifyQrDataUrl.value = certifyUrl.value
          ? await QRCode.toDataURL(certifyUrl.value, { width: 220, margin: 1 })
          : ''
        certifyDialogVisible.value = true
        startCertifyPolling()
        return
      }
      // 快瞳与靓仔人脸：生成本站拍照认证单，扫码后抓拍完成核验
      if (
        (data.data?.provider === 'kuaitong' || data.data?.provider === 'tencent') &&
        data.data?.faceToken
      ) {
        faceToken.value = data.data.faceToken
        const faceUrl = `${location.origin}/realname-face?t=${data.data.faceToken}`
        certifyUrl.value = faceUrl
        certifyQrDataUrl.value = await QRCode.toDataURL(faceUrl, { width: 220, margin: 1 })
        certifyDialogVisible.value = true
        startCertifyPolling()
        return
      }
      certifyUrl.value = data.data?.certifyUrl || ''
      certifyOuterOrderNo.value = data.data?.outerOrderNo || ''
      certifyId.value = data.data?.certifyId || ''
      certifyQrDataUrl.value = await QRCode.toDataURL(certifyUrl.value, { width: 220, margin: 1 })
      certifyDialogVisible.value = true
      startCertifyPolling()
    } catch {
      ElMessage.error('实名认证请求失败')
    } finally {
      realnameSubmitting.value = false
    }
  }

  function startCertifyPolling() {
    stopCertifyPolling()
    certifyPolling.value = true
    certifyStatusText.value = '等待认证...'
    certifyTimer = setInterval(async () => {
      try {
        const params = isTokenPollingProvider(certifyProvider.value)
          ? { faceToken: faceToken.value }
          : { outerOrderNo: certifyOuterOrderNo.value, certifyId: certifyId.value }
        const { data } = await axios.get('/api/agent-panel/realname/query', {
          params,
          headers: headers()
        })
        if (data.code === 200 && data.data?.status === 'passed') {
          stopCertifyPolling()
          certifyDialogVisible.value = false
          ElMessage.success('实名认证成功')
          resetRealnameForm()
          fetchProfile()
        } else if (data.code === 200 && data.data?.status === 'failed') {
          stopCertifyPolling()
          certifyStatusText.value = data.data.reason
            ? `认证未通过：${data.data.reason}`
            : '认证未通过，请重新提交'
        } else {
          certifyStatusText.value = '等待认证...'
        }
      } catch {
        certifyStatusText.value = '查询中...'
      }
    }, 3000)
  }

  function stopCertifyPolling() {
    certifyPolling.value = false
    if (certifyTimer) {
      clearInterval(certifyTimer)
      certifyTimer = null
    }
  }

  function openCertifyUrl() {
    if (certifyUrl.value) window.open(certifyUrl.value, '_blank')
  }

  function isFaceRealnameProvider(provider: string | undefined) {
    return provider === 'kuaitong' || provider === 'tencent'
  }

  // 认证单通过 faceToken 轮询的服务商（本站拍照单与聚合认证单）
  function isTokenPollingProvider(provider: string | undefined) {
    return isFaceRealnameProvider(provider) || provider === 'xiaomu'
  }

  const needRealnameMobile = computed(
    () => profile.realnameProvider === 'xiaomu' && profile.realnameAuthMode === 'three_element'
  )

  const realnameSubmitButtonText = computed(() => {
    if (profile.realnameProvider === 'xiaomu') {
      return profile.realnameAuthMode === 'three_element' ? '提交并核验' : '提交并跳转认证'
    }
    if (profile.realnameProvider === 'tencent') return '提交并扫码认证'
    return isFaceRealnameProvider(profile.realnameProvider)
      ? '提交并扫码认证'
      : '提交并进行支付宝认证'
  })

  const certifyDialogTitle = computed(() => {
    if (certifyProvider.value === 'xiaomu') {
      return '实名认证'
    }
    return isFaceRealnameProvider(certifyProvider.value) ? '扫码人脸核验' : '支付宝扫码认证'
  })

  const certifyDialogTip = computed(() => {
    if (certifyProvider.value === 'xiaomu') {
      return '请扫码或点击下方链接完成实名认证'
    }
    return isFaceRealnameProvider(certifyProvider.value)
      ? '请使用微信或支付宝扫码完成人脸核验'
      : '请使用支付宝 App 扫码认证'
  })

  function resetRealnameForm() {
    Object.assign(realnameForm, { realName: '', idCard: '', mobile: '' })
  }

  onMounted(fetchProfile)
  onBeforeUnmount(stopCertifyPolling)
</script>

<style scoped lang="scss">
  .panel-profile {
    background: var(--el-bg-color);

    .art-card {
      overflow: hidden;
      background: var(--el-bg-color);
      border-radius: 12px !important;
    }
  }

  .profile-hero {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 20px;

    .hero-left {
      display: flex;
      align-items: center;
      gap: 20px;
    }

    .hero-avatar {
      background: var(--el-color-primary-light-7);
      color: var(--el-color-primary);
      flex-shrink: 0;
    }

    .hero-info {
      .hero-name {
        font-size: 20px;
        font-weight: 600;
        margin-bottom: 6px;
      }

      .hero-meta {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 4px;

        .hero-id {
          font-size: 13px;
          color: var(--el-text-color-secondary);
        }
      }

      .hero-desc {
        font-size: 13px;
        color: var(--el-text-color-secondary);
        margin: 0;
      }
    }

    .hero-stats {
      display: flex;
      gap: 32px;

      .hero-stat-item {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 4px;

        .stat-num {
          font-size: 20px;
          font-weight: 700;
          color: var(--el-color-primary);
        }

        .stat-label {
          font-size: 12px;
          color: var(--el-text-color-secondary);
        }
      }
    }
  }

  .section-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding-bottom: 12px;
    border-bottom: 1px solid var(--el-border-color-lighter);

    .section-icon {
      color: var(--el-color-primary);
    }

    h4 {
      margin: 0;
      font-size: 16px;
      font-weight: 600;
    }
  }

  .mt-4 {
    margin-top: 16px;
  }
  .mr-1 {
    margin-right: 4px;
  }

  .security-tips {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    line-height: 1.5;
  }

  .realname-card {
    margin-bottom: 20px;
  }
  .realname-content,
  .realname-complete {
    padding-top: 18px;
  }
  .realname-form {
    max-width: 520px;
    margin-top: 18px;
  }

  .realname-mobile-tip {
    margin-top: 4px;
    font-size: 12px;
    line-height: 1.5;
    color: var(--el-text-color-secondary);
  }
  .verified-title {
    display: flex;
    align-items: center;
    gap: 7px;
    color: var(--el-color-success);
    font-size: 15px;
    font-weight: 600;
  }
  .realname-details {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 12px;
    margin-top: 16px;
  }
  .realname-details > div {
    display: flex;
    flex-direction: column;
    gap: 5px;
    padding: 14px 16px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
  }
  .realname-details span {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
  .realname-details strong {
    font-size: 14px;
    color: var(--el-text-color-primary);
  }
  .certify-dialog {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 8px 0;
  }
  .certify-qr {
    width: 220px;
    height: 220px;
    margin-bottom: 12px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;
  }
  .certify-tip {
    margin: 0 0 4px;
    font-size: 14px;
    color: var(--el-text-color-regular);
  }
  .certify-status {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 16px;
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }

  @media (max-width: 768px) {
    .realname-details {
      grid-template-columns: 1fr;
    }
  }
</style>
