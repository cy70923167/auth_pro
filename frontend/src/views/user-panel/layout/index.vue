<template>
  <div class="user-panel-layout">
    <aside class="panel-sidebar">
      <div class="sidebar-header">
        <img :src="resolvedLogo" class="brand-logo" alt="logo" />
        <span class="brand-text" v-show="!collapsed">{{ siteName }}</span>
      </div>

      <el-menu
        :default-active="currentRoute"
        :collapse="collapsed"
        router
        class="sidebar-menu"
      >
        <el-menu-item index="/user/dashboard">
          <el-icon><iconify-icon icon="ri:dashboard-line" /></el-icon>
          <template #title>概览</template>
        </el-menu-item>
        <el-menu-item index="/user/licenses">
          <el-icon><iconify-icon icon="ri:shield-check-line" /></el-icon>
          <template #title>我的授权</template>
        </el-menu-item>
        <el-menu-item v-if="selfPurchaseEnabled" index="/user/purchase">
          <el-icon><iconify-icon icon="ri:shopping-cart-2-line" /></el-icon>
          <template #title>购买授权</template>
        </el-menu-item>
        <el-menu-item index="/user/profile">
          <el-icon><iconify-icon icon="ri:settings-3-line" /></el-icon>
          <template #title>个人设置</template>
        </el-menu-item>
      </el-menu>
    </aside>

    <div class="panel-main">
      <header class="panel-header">
        <div class="header-left">
          <el-icon class="collapse-btn" @click="collapsed = !collapsed" :size="18">
            <iconify-icon :icon="collapsed ? 'ri:menu-unfold-line' : 'ri:menu-fold-line'" />
          </el-icon>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item>{{ siteName }}</el-breadcrumb-item>
            <el-breadcrumb-item>{{ currentTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <PanelThemeToggle scope="user" />
          <span class="header-balance">
            <iconify-icon icon="ri:wallet-3-line" width="16" />
            ¥{{ balance.toFixed(2) }}
          </span>
          <span class="user-name">{{ nickname }}</span>
          <el-dropdown trigger="click">
            <el-avatar :size="32" class="avatar-btn">
              <iconify-icon icon="ri:user-3-fill" width="18" />
            </el-avatar>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="handleLogout">
                  <el-icon><iconify-icon icon="ri:logout-box-r-line" /></el-icon>
                  退出登录
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main
        class="panel-content"
        :class="{
          'is-panel-surface': ['/user/dashboard', '/user/profile'].includes(currentRoute)
        }"
      >
        <router-view />
      </main>
      <footer v-if="icpNumber" class="panel-footer">
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">
          {{ icpNumber }}
        </a>
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Icon as IconifyIcon } from '@iconify/vue'
import axios from 'axios'
import { useSystemConfigStore } from '@/store/modules/system-config'
import PanelThemeToggle from '@/components/core/theme/PanelThemeToggle.vue'

const route = useRoute()
const router = useRouter()
const systemConfigStore = useSystemConfigStore()
const { siteName, resolvedLogo, selfPurchaseEnabled, icpNumber } =
  storeToRefs(systemConfigStore)
const collapsed = ref(false)
const balance = ref(0)
const nickname = ref('')

function getToken() {
  return localStorage.getItem('user_panel_token') || ''
}

function loadUserInfo() {
  try {
    const info = JSON.parse(localStorage.getItem('user_panel_info') || '{}')
    nickname.value = info.nickname || info.email || '用户'
  } catch {
    nickname.value = '用户'
  }
}

async function fetchBalance() {
  try {
    const { data } = await axios.get('/api/user-panel/balance', {
      headers: { Authorization: `Bearer ${getToken()}` }
    })
    if (data.code === 200) {
      balance.value = data.data.balance || 0
    }
  } catch {}
}

onMounted(() => {
  loadUserInfo()
  fetchBalance()
  window.addEventListener('user-panel-balance-refresh', fetchBalance)
})

onBeforeUnmount(() => {
  window.removeEventListener('user-panel-balance-refresh', fetchBalance)
})

watch(
  selfPurchaseEnabled,
  (enabled) => {
    if (!enabled && route.path === '/user/purchase') router.replace('/user/dashboard')
  },
  { immediate: true }
)

const currentRoute = computed(() => route.path)

const titleMap: Record<string, string> = {
  '/user/dashboard': '概览',
  '/user/licenses': '我的授权',
  '/user/purchase': '购买授权',
  '/user/profile': '个人设置'
}

const currentTitle = computed(() => titleMap[route.path] || '概览')

function handleLogout() {
  localStorage.removeItem('user_panel_token')
  localStorage.removeItem('user_panel_info')
  router.push('/user/login')
}
</script>

<style scoped lang="scss">
.user-panel-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
  background: var(--el-bg-color);
}

.panel-sidebar {
  width: 220px;
  background: var(--el-bg-color);
  display: flex;
  flex-direction: column;
  transition: width 0.3s;
  flex-shrink: 0;
  border-right: 1px solid var(--el-border-color-lighter);
  box-shadow: 2px 0 8px rgba(0, 0, 0, 0.03);

  .sidebar-header {
    height: 56px;
    display: flex;
    align-items: center;
    padding: 0 16px;
    gap: 10px;
    border-bottom: 1px solid var(--el-border-color-lighter);

    .brand-logo {
      width: 32px;
      height: 32px;
      border-radius: 6px;
      flex-shrink: 0;
    }

    .brand-text {
      font-size: 15px;
      font-weight: 600;
      color: var(--el-text-color-primary);
      white-space: nowrap;
    }
  }

  .sidebar-menu {
    border-right: none;
    flex: 1;
    padding: 8px 0;

    :deep(.el-menu-item) {
      margin: 2px 8px;
      border-radius: 8px;
      height: 44px;

      &.is-active {
        background: var(--el-color-primary-light-9);
        color: var(--el-color-primary);
      }
    }
  }
}

.panel-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.panel-header {
  height: 56px;
  background: var(--el-bg-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  flex-shrink: 0;

  .header-left {
    display: flex;
    align-items: center;
    gap: 16px;

    .collapse-btn {
      cursor: pointer;
      color: var(--el-text-color-secondary);
      transition: color 0.2s;
      &:hover { color: var(--el-color-primary); }
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 12px;

    .user-name {
      font-size: 14px;
      color: var(--el-text-color-primary);
    }

    .header-balance {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: 14px;
      font-weight: 600;
      font-family: 'DIN Alternate', 'Roboto Mono', monospace;
      color: var(--el-color-success);
      background: var(--el-color-success-light-9);
      padding: 4px 10px;
      border-radius: 6px;
    }

    .avatar-btn {
      cursor: pointer;
      background: var(--el-color-primary-light-7);
      color: var(--el-color-primary);
    }
  }
}

.panel-content {
  flex: 1;
  padding: 16px;
  overflow: auto;
  background: var(--el-bg-color-page);

  &.is-panel-surface {
    background: var(--el-bg-color);
  }
}

.panel-footer {
  flex-shrink: 0;
  padding: 8px 16px 10px;
  font-size: 12px;
  text-align: center;
  border-top: 1px solid var(--el-border-color-lighter);

  a {
    color: var(--el-text-color-placeholder);
    text-decoration: none;

    &:hover {
      color: var(--el-color-primary);
    }
  }
}
</style>