<template>
  <div class="install-page">
    <div class="install-container">
      <div class="install-header">
        <h1 class="install-title">系统安装向导</h1>
        <p class="install-desc">首次使用，请完成以下配置</p>
      </div>

      <!-- 步骤指示器 -->
      <div class="steps-bar">
        <div class="step" :class="{ active: step >= 1, done: step > 1 }">
          <div class="step-dot">1</div>
          <span class="step-text">数据库配置</span>
        </div>
        <div class="step-line" :class="{ active: step >= 2 }"></div>
        <div class="step" :class="{ active: step >= 2, done: step > 2 }">
          <div class="step-dot">2</div>
          <span class="step-text">安装数据表</span>
        </div>
        <div class="step-line" :class="{ active: step >= 3 }"></div>
        <div class="step" :class="{ active: step >= 3 }">
          <div class="step-dot">3</div>
          <span class="step-text">管理员配置</span>
        </div>
      </div>

      <transition name="fade" mode="out-in">
        <!-- Step 1: 数据库配置 -->
        <div v-if="step === 1" key="step1" class="step-content">
          <el-form :model="dbForm" label-position="top" class="install-form">
            <el-form-item label="数据库地址">
              <el-input v-model="dbForm.host" placeholder="127.0.0.1" />
            </el-form-item>
            <el-form-item label="端口">
              <el-input v-model="dbForm.port" placeholder="3306" />
            </el-form-item>
            <el-form-item label="数据库名">
              <el-input v-model="dbForm.database" placeholder="auth_pro" />
            </el-form-item>
            <el-form-item label="用户名">
              <el-input v-model="dbForm.username" placeholder="请输入数据库用户名" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="dbForm.password" type="password" show-password placeholder="请输入数据库密码" />
            </el-form-item>
          </el-form>

          <div class="action-row">
            <el-button :loading="testing" @click="handleTestConnection">
              <IconifyIcon icon="ri:link" width="16" style="margin-right: 4px" />
              测试连接
            </el-button>
            <el-button v-if="dbConnected" type="primary" @click="step = 2; handleInstallTables()">
              下一步
            </el-button>
          </div>

          <div v-if="dbConnected" class="test-result success">
            <IconifyIcon icon="ri:checkbox-circle-fill" width="16" />
            数据库连接成功
          </div>
          <div v-if="testError" class="test-result error">
            <IconifyIcon icon="ri:close-circle-fill" width="16" />
            {{ testError }}
          </div>
        </div>

        <!-- Step 2: 安装数据表 -->
        <div v-else-if="step === 2" key="step2" class="step-content">
          <div class="install-progress">
            <div v-if="installing" class="progress-info">
              <el-icon class="is-loading" :size="24"><IconifyIcon icon="ri:loader-4-line" /></el-icon>
              <p>正在安装数据表，请稍候...</p>
            </div>
            <div v-else-if="installSuccess" class="progress-info success">
              <IconifyIcon icon="ri:checkbox-circle-fill" width="40" color="#67c23a" />
              <p>数据表安装完成</p>
              <el-button type="primary" @click="step = 3" style="margin-top: 16px">下一步</el-button>
            </div>
            <div v-else-if="installError" class="progress-info error">
              <IconifyIcon icon="ri:close-circle-fill" width="40" color="#f56c6c" />
              <p>安装失败：{{ installError }}</p>
              <el-button @click="step = 1" style="margin-top: 16px">返回上一步</el-button>
            </div>
          </div>
        </div>

        <!-- Step 3: 管理员配置 -->
        <div v-else-if="step === 3" key="step3" class="step-content">
          <el-form :model="adminForm" label-position="top" class="install-form">
            <el-form-item label="管理员账号">
              <el-input v-model="adminForm.username" placeholder="admin" />
            </el-form-item>
            <el-form-item label="管理员密码">
              <el-input v-model="adminForm.password" type="password" show-password placeholder="123456" />
            </el-form-item>
            <el-form-item label="确认密码">
              <el-input v-model="adminForm.confirmPassword" type="password" show-password placeholder="123456" />
            </el-form-item>
          </el-form>

          <div class="action-row">
            <el-button type="primary" :loading="saving" @click="handleSaveAdmin">
              保存并完成安装
            </el-button>
          </div>
        </div>

        <!-- Step 4: 安装完成 -->
        <div v-else-if="step === 4" key="step4" class="step-content">
          <div class="install-done">
            <IconifyIcon icon="ri:checkbox-circle-fill" width="56" color="#67c23a" />
            <h3 class="done-title">系统安装完成</h3>
            <p class="done-desc">管理员账号已创建，请选择要访问的面板</p>
            <div class="done-buttons">
              <el-button type="primary" size="large" @click="router.push('/user')">
                <IconifyIcon icon="ri:user-3-line" width="18" style="margin-right: 6px" />
                用户端
              </el-button>
              <el-button type="success" size="large" @click="router.push('/agent')">
                <IconifyIcon icon="ri:team-line" width="18" style="margin-right: 6px" />
                代理端
              </el-button>
              <el-button type="warning" size="large" @click="router.replace('/admin')">
                <IconifyIcon icon="ri:admin-line" width="18" style="margin-right: 6px" />
                管理员面板
              </el-button>
            </div>
          </div>
        </div>
      </transition>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Icon as IconifyIcon } from '@iconify/vue'

const router = useRouter()
const step = ref(1)
const testing = ref(false)
const dbConnected = ref(false)
const testError = ref('')
const installing = ref(false)
const installSuccess = ref(false)
const installError = ref('')
const saving = ref(false)

const dbForm = reactive({
  host: '127.0.0.1',
  port: '3306',
  database: 'auth_pro',
  username: '',
  password: ''
})

const adminForm = reactive({
  username: 'admin',
  password: '123456',
  confirmPassword: '123456'
})

async function postJSON(url: string, data: any) {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data)
  })
  const json = await res.json()
  if (json.code !== 200) throw new Error(json.message || '请求失败')
  return json
}

async function handleTestConnection() {
  testing.value = true
  dbConnected.value = false
  testError.value = ''
  try {
    await postJSON('/api/install/test-db', dbForm)
    dbConnected.value = true
  } catch (e: any) {
    testError.value = e?.message || '连接失败，请检查配置'
  } finally {
    testing.value = false
  }
}

async function handleInstallTables() {
  installing.value = true
  installSuccess.value = false
  installError.value = ''
  try {
    await postJSON('/api/install/init-tables', dbForm)
    installSuccess.value = true
  } catch (e: any) {
    installError.value = e?.message || '安装失败'
  } finally {
    installing.value = false
  }
}

async function handleSaveAdmin() {
  if (adminForm.password !== adminForm.confirmPassword) {
    ElMessage.warning('两次密码输入不一致')
    return
  }
  saving.value = true
  try {
    await postJSON('/api/install/create-admin', {
      ...dbForm,
      adminUsername: adminForm.username,
      adminPassword: adminForm.password
    })
    ElMessage.success('安装完成！')
    step.value = 4
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped lang="scss">
.install-page {
  --el-bg-color: #fff;
  --el-bg-color-overlay: #fff;
  --el-fill-color: #f0f2f5;
  --el-fill-color-light: #f5f7fa;
  --el-fill-color-lighter: #fafafa;
  --el-fill-color-blank: #fff;
  --el-text-color-primary: #303133;
  --el-text-color-regular: #606266;
  --el-text-color-secondary: #909399;
  --el-text-color-placeholder: #a8abb2;
  --el-border-color: #dcdfe6;
  --el-border-color-light: #e4e7ed;
  --el-border-color-lighter: #ebeef5;

  width: 100%;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-primary);
  background: #f5f7fa;
  padding: 24px;
}

.install-container {
  width: 520px;
  background: #fff;
  border-radius: 16px;
  padding: 40px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.06);
  border: 1px solid var(--el-border-color-lighter);
}

.install-header {
  text-align: center;
  margin-bottom: 32px;

  .install-title {
    font-size: 24px;
    font-weight: 700;
    color: #1a1a1a;
    margin-bottom: 8px;
  }

  .install-desc {
    font-size: 14px;
    color: #999;
  }
}

.steps-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 32px;

  .step {
    display: flex;
    align-items: center;
    gap: 6px;

    .step-dot {
      width: 26px;
      height: 26px;
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
      font-size: 12px;
      color: var(--el-text-color-secondary);
      font-weight: 500;
    }

    &.active .step-dot {
      background: var(--el-color-primary);
      color: #fff;
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
    width: 40px;
    height: 2px;
    background: var(--el-fill-color);
    margin: 0 8px;
    transition: background 0.3s;

    &.active {
      background: var(--el-color-primary);
    }
  }
}

.install-form {
  :deep(.el-form-item__label) {
    color: var(--el-text-color-regular);
    font-weight: 500;
  }

  :deep(.el-input) {
    --el-input-text-color: #606266;
    --el-input-bg-color: #fff;
    --el-input-icon-color: #a8abb2;
    --el-input-placeholder-color: #a8abb2;
    --el-text-color-regular: #606266;
    --el-text-color-placeholder: #a8abb2;
  }

  :deep(.el-input__wrapper) {
    background-color: #fff;
  }
}

.action-row {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.test-result {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 13px;

  &.success { color: #67c23a; }
  &.error { color: #f56c6c; }
}

.install-progress {
  text-align: center;
  padding: 40px 0;

  .progress-info {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;

    p {
      font-size: 15px;
      color: var(--el-text-color-primary);
    }
  }
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s, transform 0.2s;
}
.fade-enter-from { opacity: 0; transform: translateX(12px); }
.fade-leave-to { opacity: 0; transform: translateX(-12px); }

.install-done {
  text-align: center;
  padding: 32px 0;

  .done-title {
    font-size: 20px;
    font-weight: 700;
    color: var(--el-text-color-primary);
    margin: 16px 0 8px;
  }

  .done-desc {
    font-size: 14px;
    color: var(--el-text-color-secondary);
    margin-bottom: 28px;
  }

  .done-buttons {
    display: flex;
    gap: 12px;
    justify-content: center;
    flex-wrap: wrap;
  }
}
</style>
