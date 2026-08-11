<template>
  <el-tooltip :content="isDark ? '切换为亮色主题' : '切换为暗色主题'" placement="bottom">
    <button
      type="button"
      class="panel-theme-toggle"
      :aria-label="isDark ? '切换为亮色主题' : '切换为暗色主题'"
      @click="toggleTheme"
    >
      <IconifyIcon :icon="isDark ? 'ri:sun-fill' : 'ri:moon-line'" width="18" />
    </button>
  </el-tooltip>
</template>

<script setup lang="ts">
  import { Icon as IconifyIcon } from '@iconify/vue'
  import { useScopedTheme, type PanelThemeScope } from '@/hooks/core/useTheme'

  const props = defineProps<{
    scope: PanelThemeScope
  }>()

  const { isDark, toggleTheme } = useScopedTheme(props.scope)
</script>

<style scoped lang="scss">
  .panel-theme-toggle {
    display: inline-flex;
    width: 32px;
    height: 32px;
    padding: 0;
    color: var(--el-text-color-regular);
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: 8px;
    align-items: center;
    justify-content: center;
    transition:
      color 0.2s,
      background-color 0.2s;

    &:hover {
      color: var(--el-color-primary);
      background: var(--el-fill-color-light);
    }

    &:focus-visible {
      outline: 2px solid var(--el-color-primary-light-5);
      outline-offset: 2px;
    }
  }
</style>
