/**
 * useTheme - 系统主题管理
 *
 * 提供完整的主题切换和管理功能，支持亮色、暗色和自动模式。
 * 自动处理主题切换时的过渡效果，确保切换流畅无闪烁。
 *
 * ## 主要功能
 *
 * 1. 主题切换 - 支持亮色、暗色、自动三种主题模式
 * 2. 自动模式 - 根据系统偏好自动切换主题
 * 3. 颜色适配 - 自动调整主题色的明暗变体（9 个层级）
 * 4. 过渡优化 - 切换时临时禁用过渡效果，避免闪烁
 * 5. 状态持久化 - 主题设置自动保存到 store
 *
 * ## 使用示例
 *
 * ```typescript
 * const { switchThemeStyles } = useTheme()
 *
 * // 切换到暗色主题
 * switchThemeStyles(SystemThemeEnum.DARK)
 *
 * // 切换到亮色主题
 * switchThemeStyles(SystemThemeEnum.LIGHT)
 *
 * // 切换到自动模式（跟随系统）
 * switchThemeStyles(SystemThemeEnum.AUTO)
 * ```
 *
 * @module useTheme
 * @author Art Design Pro Team
 */

import { useSettingStore } from '@/store/modules/setting'
import { SystemThemeEnum } from '@/enums/appEnum'
import AppConfig from '@/config'
import { SystemThemeTypes } from '@/types/store'
import { handleElementThemeColor, setElementThemeColor } from '@/utils/ui'
import { StorageConfig } from '@/utils'
import { usePreferredDark } from '@vueuse/core'
import { computed, reactive, watch } from 'vue'

export type ThemeScope = 'admin' | 'user' | 'agent'
export type PanelThemeScope = Exclude<ThemeScope, 'admin'>

const prefersDark = usePreferredDark()
const panelThemeKeys: Record<PanelThemeScope, string> = {
  user: StorageConfig.USER_THEME_KEY,
  agent: StorageConfig.AGENT_THEME_KEY
}

const readPanelTheme = (scope: PanelThemeScope): SystemThemeEnum => {
  try {
    const theme = localStorage.getItem(panelThemeKeys[scope])
    if (Object.values(SystemThemeEnum).includes(theme as SystemThemeEnum)) {
      return theme as SystemThemeEnum
    }
  } catch {
    return SystemThemeEnum.LIGHT
  }

  return SystemThemeEnum.LIGHT
}

const panelThemeModes = reactive<Record<PanelThemeScope, SystemThemeEnum>>({
  user: readPanelTheme('user'),
  agent: readPanelTheme('agent')
})

const persistPanelTheme = (scope: PanelThemeScope, mode: SystemThemeEnum): void => {
  try {
    localStorage.setItem(panelThemeKeys[scope], mode)
  } catch {
    return
  }
}

const resolveTheme = (mode: SystemThemeEnum): SystemThemeEnum => {
  if (mode === SystemThemeEnum.AUTO) {
    return prefersDark.value ? SystemThemeEnum.DARK : SystemThemeEnum.LIGHT
  }
  return mode
}

const applyDocumentTheme = (theme: SystemThemeEnum): void => {
  const settingStore = useSettingStore()
  const isDark = theme === SystemThemeEnum.DARK
  const currentTheme = AppConfig.systemThemeStyles[theme as keyof SystemThemeTypes]

  document.documentElement.classList.toggle(
    'dark',
    currentTheme?.className === SystemThemeEnum.DARK
  )
  setElementThemeColor(settingStore.systemThemeColor)
  handleElementThemeColor(settingStore.systemThemeColor, isDark)
}

export function getThemeScope(path: string): ThemeScope {
  if (/^\/user(?:\/|$)/.test(path)) return 'user'
  if (/^\/agent(?:\/|$)/.test(path)) return 'agent'
  return 'admin'
}

const getThemeMode = (scope: ThemeScope): SystemThemeEnum => {
  if (scope === 'admin') return useSettingStore().systemThemeMode
  return panelThemeModes[scope]
}

export function applyThemeForPath(path: string = window.location.pathname): void {
  const scope = getThemeScope(path)
  const mode = getThemeMode(scope)
  const actualTheme = resolveTheme(mode)

  if (scope === 'admin' && mode === SystemThemeEnum.AUTO) {
    useSettingStore().systemThemeType = actualTheme
  }

  applyDocumentTheme(actualTheme)
}

export function useTheme() {
  const settingStore = useSettingStore()

  const disableTransitions = () => {
    const existingStyle = document.getElementById('disable-transitions')
    if (existingStyle) existingStyle.remove()

    const style = document.createElement('style')
    style.setAttribute('id', 'disable-transitions')
    style.textContent = '* { transition: none !important; }'
    document.head.appendChild(style)
  }

  const enableTransitions = () => {
    document.getElementById('disable-transitions')?.remove()
  }

  const setSystemTheme = (theme: SystemThemeEnum, themeMode: SystemThemeEnum = theme) => {
    disableTransitions()
    applyDocumentTheme(theme)
    settingStore.setGlopTheme(theme, themeMode)

    requestAnimationFrame(() => {
      requestAnimationFrame(enableTransitions)
    })
  }

  const setSystemAutoTheme = () => {
    setSystemTheme(resolveTheme(SystemThemeEnum.AUTO), SystemThemeEnum.AUTO)
  }

  const switchThemeStyles = (theme: SystemThemeEnum) => {
    if (theme === SystemThemeEnum.AUTO) {
      setSystemAutoTheme()
    } else {
      setSystemTheme(theme)
    }
  }

  return {
    setSystemTheme,
    setSystemAutoTheme,
    switchThemeStyles,
    prefersDark
  }
}

export function useScopedTheme(scope: PanelThemeScope) {
  const themeMode = computed(() => panelThemeModes[scope])
  const isDark = computed(() => resolveTheme(themeMode.value) === SystemThemeEnum.DARK)

  const setThemeMode = (mode: SystemThemeEnum): void => {
    panelThemeModes[scope] = mode
    persistPanelTheme(scope, mode)
    applyDocumentTheme(resolveTheme(mode))
  }

  const toggleTheme = (): void => {
    setThemeMode(isDark.value ? SystemThemeEnum.LIGHT : SystemThemeEnum.DARK)
  }

  return {
    themeMode,
    isDark,
    setThemeMode,
    toggleTheme
  }
}

let themeWatcherInitialized = false

export function initializeTheme(): void {
  const settingStore = useSettingStore()

  applyThemeForPath()
  document.documentElement.style.setProperty('--custom-radius', `${settingStore.customRadius}rem`)

  if (themeWatcherInitialized) return
  themeWatcherInitialized = true

  watch(prefersDark, () => {
    const scope = getThemeScope(window.location.pathname)
    if (getThemeMode(scope) === SystemThemeEnum.AUTO) {
      applyThemeForPath()
    }
  })
}
