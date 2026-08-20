import App from './App.vue'
import { createApp } from 'vue'
import { initStore } from './store'                 // Store
import { initRouter } from './router'               // Router
import language from './locales'                    // 国际化
import '@styles/core/tailwind.css'                  // tailwind
import '@styles/index.scss'                         // 样式
import '@utils/ui/iconify-loader'                   // 离线图标
import { setupGlobDirectives } from './directives'
import { setupErrorHandle } from './utils/sys/error-handle'
import { useSystemConfigStore } from './store/modules/system-config'

document.addEventListener(
  'touchstart',
  function () {},
  { passive: false }
)

// 控制台小尾巴
const consoleTail = () => {
  const style1 = 'background: linear-gradient(90deg, #667eea 0%, #764ba2 100%); color: #fff; padding: 5px 10px; border-radius: 3px 0 0 3px; font-weight: bold;'
  const style2 = 'background: linear-gradient(90deg, #764ba2 0%, #667eea 100%); color: #fff; padding: 5px 10px; border-radius: 0 3px 3px 0; font-weight: bold;'
  const style3 = 'color: #667eea; font-weight: bold; font-size: 12px;'
  const style4 = 'color: #999; font-size: 11px;'
  
  console.log('%c⚡ 授权管理系统 %c v1.1.4 ', style1, style2)
  console.log('%c本授权系统完全免费适用，谨防上当受骗', style3)
  console.log('%c官方QQ群：169484041 | 认准官方群', style3)
  console.log('%c━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━', style4)
}

consoleTail()

const bootstrap = async () => {
  const app = createApp(App)
  initStore(app)
  await useSystemConfigStore().loadPublicConfig()
  initRouter(app)
  setupGlobDirectives(app)
  setupErrorHandle(app)

  app.use(language)
  app.mount('#app')
}

void bootstrap()
