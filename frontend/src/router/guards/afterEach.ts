import { nextTick, watch } from 'vue'
import { useSettingStore } from '@/store/modules/setting'
import { useSystemConfigStore } from '@/store/modules/system-config'
import { Router } from 'vue-router'
import NProgress from 'nprogress'
import { useCommon } from '@/hooks/core/useCommon'
import { loadingService } from '@/utils/ui'
import { getPendingLoading, resetPendingLoading } from './beforeEach'
import { setPageTitle } from '@/utils/router'
import { applyThemeForPath } from '@/hooks/core/useTheme'

/** 路由全局后置守卫 */
export function setupAfterEachGuard(router: Router) {
  const { scrollToTop } = useCommon()
  const systemConfigStore = useSystemConfigStore()

  watch(
    () => systemConfigStore.siteName,
    () => setPageTitle(router.currentRoute.value),
    { immediate: true }
  )

  router.afterEach((to) => {
    applyThemeForPath(to.path)
    scrollToTop()
    setPageTitle(to)

    // 关闭进度条
    const settingStore = useSettingStore()
    if (settingStore.showNprogress) {
      NProgress.done()
      // 确保进度条完全移除，避免残影
      setTimeout(() => {
        NProgress.remove()
      }, 600)
    }

    // 关闭 loading 效果
    if (getPendingLoading()) {
      nextTick(() => {
        loadingService.hideLoading()
        resetPendingLoading()
      })
    }
  })
}
