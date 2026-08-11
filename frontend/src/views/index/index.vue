<!-- 布局容器 -->
<template>
  <div class="app-layout">
    <aside id="app-sidebar">
      <ArtSidebarMenu />
    </aside>

    <main id="app-main">
      <div id="app-header">
        <ArtHeaderBar />
      </div>
      <div id="app-content">
        <ArtPageContent />
      </div>
      <footer v-if="icpNumber" id="app-footer">
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">
          {{ icpNumber }}
        </a>
      </footer>
    </main>

    <div id="app-global">
      <ArtGlobalComponent />
    </div>
  </div>
</template>

<script setup lang="ts">
  import { useSystemConfigStore } from '@/store/modules/system-config'

  defineOptions({ name: 'AppLayout' })

  const { icpNumber } = storeToRefs(useSystemConfigStore())
</script>

<style lang="scss" scoped>
  @use './style';
</style>
