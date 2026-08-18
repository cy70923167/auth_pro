<template>
  <div class="agent-panel-layout">
    <!-- 侧边栏 -->
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
        <el-menu-item index="/agent-panel/dashboard">
          <el-icon><iconify-icon icon="ri:dashboard-line" /></el-icon>
          <template #title>概览</template>
        </el-menu-item>
        <el-menu-item index="/agent-panel/licenses">
          <el-icon><iconify-icon icon="ri:file-list-3-line" /></el-icon>
          <template #title>我的授权</template>
        </el-menu-item>
        <el-menu-item index="/agent-panel/purchase">
          <el-icon><iconify-icon icon="ri:add-circle-line" /></el-icon>
          <template #title>开通授权</template>
        </el-menu-item>
        <el-menu-item index="/agent-panel/finance">
          <el-icon><iconify-icon icon="ri:money-cny-circle-line" /></el-icon>
          <template #title>我的财务</template>
        </el-menu-item>
        <el-menu-item index="/agent-panel/profile">
          <el-icon><iconify-icon icon="ri:settings-3-line" /></el-icon>
          <template #title>个人设置</template>
        </el-menu-item>
      </el-menu>
    </aside>

    <!-- 主内容区 -->
    <div class="panel-main">
      <!-- 顶栏 -->
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
          <PanelThemeToggle scope="agent" />
          <span class="agent-name">{{ agentName }}</span>
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

      <!-- 页面内容 -->
      <main
        class="panel-content"
        :class="{
          'is-panel-surface': ['/agent-panel/dashboard', '/agent-panel/profile'].includes(currentRoute)
        }"
      >
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Icon as IconifyIcon } from '@iconify/vue'
import { useSystemConfigStore } from '@/store/modules/system-config'
import PanelThemeToggle from '@/components/core/theme/PanelThemeToggle.vue'

const route = useRoute()
const router = useRouter()
const systemConfigStore = useSystemConfigStore()
const { siteName, resolvedLogo } = storeToRefs(systemConfigStore)
const collapsed = ref(false)

const currentRoute = computed(() => route.path)

const titleMap: Record<string, string> = {
  '/agent-panel/dashboard': '概览',
  '/agent-panel/licenses': '我的授权',
  '/agent-panel/purchase': '开通授权',
  '/agent-panel/finance': '我的财务',
  '/agent-panel/profile': '个人设置'
}

const currentTitle = computed(() => titleMap[route.path] || '概览')
const agentName = computed(() => {
  try {
    const info = JSON.parse(localStorage.getItem('agent_panel_info') || '{}')
    return info.name || info.email || '代理商'
  } catch {
    return '代理商'
  }
})

function handleLogout() {
  localStorage.removeItem('agent_panel_token')
  localStorage.removeItem('agent_panel_info')
  router.push('/agent-panel/login')
}
</script>

<style scoped lang="scss">
.agent-panel-layout {
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

    .agent-name {
      font-size: 14px;
      color: var(--el-text-color-primary);
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
</style>