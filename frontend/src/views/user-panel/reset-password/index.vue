<template>
  <div class="reset-password">
    <div class="theme-toggle">
      <PanelThemeToggle scope="user" />
    </div>
    <div class="reset-card">
      <template v-if="tokenValid">
        <h2 class="form-title">重置密码</h2>
        <p class="form-subtitle">请设置您的新密码</p>

        <el-form ref="formRef" :model="form" :rules="rules" @keyup.enter="handleSubmit" class="reset-form">
          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="新密码（至少 6 位）"
              size="large"
              show-password
              prefix-icon="ri-lock-line"
            />
          </el-form-item>
          <el-form-item prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              placeholder="确认新密码"
              size="large"
              show-password
              prefix-icon="ri-lock-line"
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="large" :loading="loading" class="submit-btn" @click="handleSubmit">
              确认重置
            </el-button>
          </el-form-item>
        </el-form>

        <div class="page-footer">
          <el-link type="primary" :underline="false" @click="goLogin">返回登录</el-link>
        </div>
      </template>

      <template v-else>
        <h2 class="form-title">链接无效</h2>
        <p class="form-subtitle">重置链接缺失或已失效，请返回登录页重新申请</p>
        <el-button type="primary" size="large" class="submit-btn" @click="goLogin">返回登录</el-button>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, type FormRules } from 'element-plus'
import axios from 'axios'
import PanelThemeToggle from '@/components/core/theme/PanelThemeToggle.vue'

const router = useRouter()
const route = useRoute()

const token = computed(() => (typeof route.query.token === 'string' ? route.query.token.trim() : ''))
const tokenValid = computed(() => /^[0-9a-f]{64}$/.test(token.value))

const loading = ref(false)
const formRef = ref()
const form = reactive({
  password: '',
  confirmPassword: ''
})

const rules: FormRules = {
  password: [
    { required: true, message: '请设置新密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, callback: Function) => {
        if (value !== form.password) {
          callback(new Error('两次密码输入不一致'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ]
}

function goLogin() {
  router.replace('/user/login')
}

function handleSubmit() {
  formRef.value?.validate(async (valid: boolean) => {
    if (!valid) return
    loading.value = true
    try {
      const { data } = await axios.post('/api/user-panel/reset-password', {
        token: token.value,
        password: form.password.trim()
      })
      if (data.code === 200) {
        ElMessage.success(data.msg || '密码重置成功，请使用新密码登录')
        router.replace('/user/login')
      } else {
        ElMessage.error(data.msg || '重置失败，请重新申请')
      }
    } catch {
      ElMessage.error('网络错误，请稍后重试')
    }
    loading.value = false
  })
}
</script>

<style scoped lang="scss">
.reset-password {
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
}

.reset-card {
  width: 420px;
  max-width: 100%;
  padding: 44px 40px;
  background: var(--el-bg-color);
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 24px;
  box-shadow: 0 24px 70px rgb(30 41 59 / 10%);
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

.reset-form {
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
}

.submit-btn {
  width: 100%;
  height: 46px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 12px;
}

.page-footer {
  margin-top: 24px;
  font-size: 13px;
  text-align: center;
}

@media (max-width: 768px) {
  .reset-password {
    padding: 20px;
  }

  .reset-card {
    padding: 32px 24px;
  }
}
</style>