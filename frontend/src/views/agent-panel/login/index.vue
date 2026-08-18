<template>
  <div class="agent-login">
    <div class="theme-toggle">
      <PanelThemeToggle scope="agent" />
    </div>
    <div class="login-container">
      <div class="login-left">
        <div class="brand">
          <h1 class="brand-title">{{ siteName }}</h1>
          <p class="brand-desc">{{ siteSubtitle }}</p>
        </div>
        <div class="features">
          <div class="feature-item">
            <el-icon :size="20" color="#67c23a"><i class="ri-check-line" /></el-icon>
            <span>在线开通授权</span>
          </div>
          <div class="feature-item">
            <el-icon :size="20" color="#67c23a"><i class="ri-check-line" /></el-icon>
            <span>实时查看配额</span>
          </div>
          <div class="feature-item">
            <el-icon :size="20" color="#67c23a"><i class="ri-check-line" /></el-icon>
            <span>财务流水透明</span>
          </div>
        </div>
      </div>

      <div class="login-right">
        <transition name="card-fade" mode="out-in">
          <!-- 登录表单 -->
          <div v-if="mode === 'login'" key="login" class="login-form-wrapper">
            <h2 class="form-title">代理商登录</h2>
            <p class="form-subtitle">请输入您的账号信息</p>

            <el-form ref="loginFormRef" :model="loginForm" :rules="loginRules" @keyup.enter="handleLogin" class="login-form">
              <el-form-item prop="username">
                <el-input
                  v-model="loginForm.username"
                  placeholder="手机号 / 邮箱"
                  size="large"
                  prefix-icon="ri-user-line"
                />
              </el-form-item>
              <el-form-item prop="password">
                <el-input
                  v-model="loginForm.password"
                  type="password"
                  placeholder="登录密码"
                  size="large"
                  show-password
                  prefix-icon="ri-lock-line"
                />
              </el-form-item>
              <div class="form-options">
                <el-checkbox v-model="loginForm.remember">记住我</el-checkbox>
                <el-link type="primary" :underline="false" @click="mode = 'forgot'">忘记密码？</el-link>
              </div>
              <el-form-item>
                <el-button type="primary" size="large" :loading="loading" @click="handleLogin" class="login-btn">
                  登 录
                </el-button>
              </el-form-item>
            </el-form>

            <div class="login-footer">
              <span>还不是代理商？</span>
              <el-link type="primary" :underline="false" @click="mode = 'register'">申请注册</el-link>
            </div>
          </div>

          <!-- 注册表单 -->
          <div v-else-if="mode === 'register'" key="register" class="login-form-wrapper">
            <h2 class="form-title">代理商注册</h2>
            <p class="form-subtitle">提交注册申请，审核通过后即可使用</p>

            <el-form ref="registerFormRef" :model="registerForm" :rules="registerRules" @keyup.enter="handleRegister" class="login-form">
              <el-form-item prop="email">
                <el-input
                  v-model="registerForm.email"
                  placeholder="邮箱地址"
                  size="large"
                  prefix-icon="ri-mail-line"
                />
              </el-form-item>
              <el-form-item prop="name">
                <el-input
                  v-model="registerForm.name"
                  placeholder="代理商账号"
                  size="large"
                  prefix-icon="ri-building-2-line"
                />
              </el-form-item>
              <el-form-item prop="password">
                <el-input
                  v-model="registerForm.password"
                  type="password"
                  placeholder="设置密码"
                  size="large"
                  show-password
                  prefix-icon="ri-lock-line"
                />
              </el-form-item>
              <el-form-item prop="confirmPassword">
                <el-input
                  v-model="registerForm.confirmPassword"
                  type="password"
                  placeholder="确认密码"
                  size="large"
                  show-password
                  prefix-icon="ri-lock-line"
                />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" size="large" :loading="loading" @click="handleRegister" class="login-btn">
                  提交注册
                </el-button>
              </el-form-item>
            </el-form>

            <div class="login-footer">
              <span>已有账号？</span>
              <el-link type="primary" :underline="false" @click="mode = 'login'">返回登录</el-link>
            </div>
          </div>

          <!-- 忘记密码 -->
          <div v-else key="forgot" class="login-form-wrapper">
            <h2 class="form-title">找回密码</h2>
            <p class="form-subtitle">输入注册邮箱，我们将发送重置链接</p>

            <el-form ref="forgotFormRef" :model="forgotForm" :rules="forgotRules" @keyup.enter="handleForgot" class="login-form">
              <el-form-item prop="email">
                <el-input
                  v-model="forgotForm.email"
                  placeholder="注册邮箱"
                  size="large"
                  prefix-icon="ri-mail-line"
                />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" size="large" :loading="loading" @click="handleForgot" class="login-btn">
                  发送重置链接
                </el-button>
              </el-form-item>
            </el-form>

            <div class="login-footer">
              <span>想起密码了？</span>
              <el-link type="primary" :underline="false" @click="mode = 'login'">返回登录</el-link>
            </div>
          </div>
        </transition>
      </div>
    </div>

    <el-dialog
      v-model="announcementVisible"
      title="网站公告"
      width="min(480px, calc(100vw - 32px))"
      align-center
      append-to-body
      @closed="handleAnnouncementClosed"
    >
      <div class="announcement-content">{{ domainLicenseNotice }}</div>
      <template #footer>
        <div class="announcement-footer">
          <el-checkbox v-model="hideAnnouncementToday">今日不再显示</el-checkbox>
          <el-button type="primary" @click="announcementVisible = false">我知道了</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, type FormRules } from 'element-plus'
import axios from 'axios'
import { useSystemConfigStore } from '@/store/modules/system-config'
import PanelThemeToggle from '@/components/core/theme/PanelThemeToggle.vue'

const router = useRouter()
const route = useRoute()

// 管理员代登录：从 sessionStorage 接管 token（仅当前标签页，不影响其他登录态）
onMounted(() => {
  if (route.query.impersonate !== '1') return
  const token = sessionStorage.getItem('impersonate_agent_token')
  const info = sessionStorage.getItem('impersonate_agent_info')
  if (!token) return
  localStorage.setItem('agent_panel_token', token)
  if (info) localStorage.setItem('agent_panel_info', info)
  sessionStorage.removeItem('impersonate_agent_token')
  sessionStorage.removeItem('impersonate_agent_info')
  ElMessage.success('已登录该代理商账号')
  router.replace('/agent-panel/dashboard')
})
const systemConfigStore = useSystemConfigStore()
const { siteName, siteSubtitle, domainLicenseNotice } = storeToRefs(systemConfigStore)
const loading = ref(false)
const mode = ref<'login' | 'register' | 'forgot'>('login')
const loginFormRef = ref()
const registerFormRef = ref()
const forgotFormRef = ref()
const announcementVisible = ref(false)
const hideAnnouncementToday = ref(false)
const ANNOUNCEMENT_HIDE_DATE_KEY = 'agent-panel-announcement-hidden-date'

function getLocalDateKey() {
  const today = new Date()
  const year = today.getFullYear()
  const month = String(today.getMonth() + 1).padStart(2, '0')
  const day = String(today.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function isAnnouncementHiddenToday() {
  try {
    return localStorage.getItem(ANNOUNCEMENT_HIDE_DATE_KEY) === getLocalDateKey()
  } catch {
    return false
  }
}

function updateAnnouncementVisibility(notice: string) {
  hideAnnouncementToday.value = false
  announcementVisible.value = Boolean(notice.trim()) && !isAnnouncementHiddenToday()
}

function handleAnnouncementClosed() {
  if (!hideAnnouncementToday.value) return
  try {
    localStorage.setItem(ANNOUNCEMENT_HIDE_DATE_KEY, getLocalDateKey())
  } catch {}
}

watch(domainLicenseNotice, updateAnnouncementVisibility, { immediate: true })

const loginForm = reactive({
  username: '',
  password: '',
  remember: false
})

const loginRules = {
  username: [{ required: true, message: '请输入手机号或邮箱', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const registerForm = reactive({
  email: '',
  name: '',
  password: '',
  confirmPassword: ''
})

const registerRules: FormRules = {
  email: [
    { required: true, message: '请输入邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ],
  name: [{ required: true, message: '请输入代理商账号', trigger: 'blur' }],
  password: [
    { required: true, message: '请设置密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, callback: Function) => {
        if (value !== registerForm.password) {
          callback(new Error('两次密码输入不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

const forgotForm = reactive({
  email: ''
})

const forgotRules: FormRules = {
  email: [
    { required: true, message: '请输入注册邮箱', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
  ]
}

function handleLogin() {
  loginFormRef.value?.validate(async (valid: boolean) => {
    if (!valid) return
    loading.value = true
    try {
      const { data } = await axios.post('/api/agent-panel/login', {
        username: loginForm.username,
        password: loginForm.password
      })
      if (data.code === 200) {
        localStorage.setItem('agent_panel_token', data.data.accessToken)
        localStorage.setItem('agent_panel_info', JSON.stringify({
          agentId: data.data.agentId,
          email: data.data.email,
          name: data.data.name,
          balance: data.data.balance
        }))
        ElMessage.success('登录成功')
        router.push('/agent-panel/dashboard')
      } else {
        ElMessage.error(data.msg || '登录失败')
      }
    } catch {
      ElMessage.error('请求失败，请重试')
    } finally {
      loading.value = false
    }
  })
}

function handleRegister() {
  registerFormRef.value?.validate((valid: boolean) => {
    if (!valid) return
    loading.value = true
    setTimeout(() => {
      loading.value = false
      ElMessage.success('注册申请已提交，请等待审核')
      mode.value = 'login'
    }, 1000)
  })
}

function handleForgot() {
  forgotFormRef.value?.validate((valid: boolean) => {
    if (!valid) return
    loading.value = true
    setTimeout(() => {
      loading.value = false
      ElMessage.success('重置链接已发送到您的邮箱')
      mode.value = 'login'
    }, 1000)
  })
}
</script>

<style scoped lang="scss">
.agent-login {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 100vh;
  padding: 32px;
  background:
    radial-gradient(circle at 20% 20%, rgb(64 158 255 / 8%), transparent 28%),
    linear-gradient(180deg, var(--el-bg-color) 0%, var(--el-bg-color-page) 100%);
}

.theme-toggle {
  position: absolute;
  top: 20px;
  right: 24px;
  z-index: 2;
}

.login-container {
  display: flex;
  width: 880px;
  min-height: 540px;
  overflow: hidden;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 24px;
  box-shadow: 0 24px 70px rgb(30 41 59 / 10%);
}

.login-left {
  display: flex;
  flex-direction: column;
  justify-content: center;
  width: 360px;
  padding: 52px 40px;
  color: var(--el-text-color-primary);
  background: var(--el-fill-color-light);
  border-right: 1px solid var(--el-border-color-lighter);

  .brand {
    margin-bottom: 36px;

    .brand-title {
      margin-bottom: 10px;
      font-size: 25px;
      font-weight: 700;
      color: var(--el-text-color-primary);
      letter-spacing: 0.2px;
    }

    .brand-desc {
      font-size: 14px;
      color: var(--el-text-color-secondary);
    }
  }

  .features {
    display: grid;
    gap: 14px;

    .feature-item {
      display: flex;
      gap: 10px;
      align-items: center;
      padding: 12px 14px;
      font-size: 14px;
      color: var(--el-text-color-regular);
      background: var(--el-bg-color);
      border: 1px solid var(--el-border-color-lighter);
      border-radius: 14px;
    }
  }
}

.login-right {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  padding: 52px;
  background: var(--el-bg-color);

  .login-form-wrapper {
    width: 100%;
    max-width: 352px;
  }

  .form-title {
    margin-bottom: 8px;
    font-size: 24px;
    font-weight: 700;
    color: var(--el-text-color-primary);
  }

  .form-subtitle {
    margin-bottom: 30px;
    font-size: 14px;
    color: var(--el-text-color-secondary);
  }

  .login-form {
    :deep(.el-input__wrapper) {
      min-height: 46px;
      background: var(--el-fill-color-lighter);
      border-radius: 12px;
      box-shadow: 0 0 0 1px var(--el-border-color-lighter) inset;
    }

    :deep(.el-input__wrapper.is-focus) {
      background: var(--el-bg-color);
      box-shadow: 0 0 0 1px var(--el-color-primary) inset;
    }

    .form-options {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 24px;
    }

    .login-btn {
      width: 100%;
      height: 46px;
      font-size: 16px;
      font-weight: 600;
      border-radius: 12px;
    }
  }

  .login-footer {
    margin-top: 24px;
    font-size: 13px;
    color: var(--el-text-color-secondary);
    text-align: center;
  }
}

.card-fade-enter-active,
.card-fade-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.announcement-content {
  max-height: min(55vh, 360px);
  padding: 4px 2px;
  overflow-y: auto;
  font-size: 14px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.announcement-footer {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
}

.card-fade-enter-from {
  opacity: 0;
  transform: translateX(20px);
}

.card-fade-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

@media (max-width: 768px) {
  .agent-login {
    padding: 20px;
  }

  .login-left {
    display: none;
  }

  .login-container {
    width: 100%;
    min-height: auto;
  }

  .login-right {
    padding: 36px 24px;
  }
}
</style>