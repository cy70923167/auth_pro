import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { fetchPublicSystemConfig, SystemConfigData } from '@/api/system-manage'
import defaultLogo from '../../../../哈士奇.svg'

const DEFAULT_SITE_NAME = '授权管理系统'
const DEFAULT_SITE_SUBTITLE = '专业的软件授权与服务平台'

export const useSystemConfigStore = defineStore(
  'systemConfigStore',
  () => {
    const siteName = ref(DEFAULT_SITE_NAME)
    const siteSubtitle = ref(DEFAULT_SITE_SUBTITLE)
    const siteLogo = ref('')
    const installedAt = ref('')
    const stationQQ = ref('')
    const icpNumber = ref('')
    const domainLicenseNotice = ref('')
    const registrationEnabled = ref(true)
    const selfPurchaseEnabled = ref(true)
    const loaded = ref(false)

    const resolvedLogo = computed(() => siteLogo.value || defaultLogo)

    const applyConfig = (config?: Partial<SystemConfigData>) => {
      siteName.value = config?.siteName?.trim() || DEFAULT_SITE_NAME
      siteSubtitle.value = config?.siteSubtitle?.trim() || DEFAULT_SITE_SUBTITLE
      siteLogo.value = config?.siteLogo || ''
      installedAt.value = config?.installedAt || ''
      stationQQ.value = config?.stationQQ?.trim() || ''
      icpNumber.value = config?.icpNumber?.trim() || ''
      domainLicenseNotice.value = config?.domainLicenseNotice?.trim() || ''
      registrationEnabled.value = config?.registrationEnabled ?? true
      selfPurchaseEnabled.value = config?.selfPurchaseEnabled ?? true
      loaded.value = true
    }

    const loadPublicConfig = async () => {
      try {
        const data = await fetchPublicSystemConfig()
        applyConfig(data)
      } catch {
        applyConfig()
      }
    }

    return {
      siteName,
      siteSubtitle,
      siteLogo,
      installedAt,
      stationQQ,
      icpNumber,
      domainLicenseNotice,
      registrationEnabled,
      selfPurchaseEnabled,
      loaded,
      resolvedLogo,
      applyConfig,
      loadPublicConfig
    }
  },
  {
    persist: {
      key: 'system-config',
      storage: localStorage
    }
  }
)
