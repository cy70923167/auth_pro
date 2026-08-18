<template>
  <div class="plugin-store">
    <ElCard shadow="never" class="art-table-card">
      <div class="store-header">
        <div>
          <h2 class="store-title">应用商店</h2>
          <p class="store-subtitle">按分区浏览插件，支持从软件源下载远程插件</p>
        </div>
        <div class="store-header-actions">
          <ElButton :icon="FolderAdd" @click="sourceDialogVisible = true">软件源管理</ElButton>
          <ElButton :icon="Refresh" circle :loading="loading" @click="loadPlugins" />
        </div>
      </div>

      <div class="store-toolbar">
        <ElTabs v-model="activeTab" class="store-tabs" @tab-change="loadPlugins">
          <ElTabPane label="全部插件" name="all" />
          <ElTabPane label="支付插件" name="payment" />
          <ElTabPane label="实名认证服务商" name="realname" />
          <ElTabPane label="其他插件" name="other" />
        </ElTabs>
        <ElInput
          v-model="searchText"
          placeholder="搜索插件名称或描述"
          :prefix-icon="Search"
          clearable
          class="store-search"
          @input="handleSearchInput"
          @clear="loadPlugins"
        />
      </div>

      <div v-loading="loading" class="store-body">
        <ElEmpty v-if="!visibleGroups.length" description="没有匹配的插件" />
        <section v-for="group in visibleGroups" :key="group.category" class="plugin-section">
          <div class="section-header">
            <strong>{{ group.title }}</strong>
            <ElText type="info" size="small">{{ group.plugins.length }} 个插件</ElText>
          </div>

          <div class="plugin-grid">
            <div
              v-for="plugin in group.plugins"
              :key="plugin.id"
              class="plugin-card"
              :class="{ 'is-enabled': plugin.enabled }"
            >
              <div class="plugin-card-top">
                <div class="plugin-icon">
                  <ArtSvgIcon :icon="plugin.icon" />
                </div>
                <div class="plugin-meta">
                  <div class="plugin-name">
                    <strong>{{ plugin.name }}</strong>
                    <ElTag v-if="plugin.official" type="primary" size="small" effect="plain"
                      >官方</ElTag
                    >
                    <ElTag type="info" size="small" effect="plain">v{{ plugin.version }}</ElTag>
                  </div>
                  <p class="plugin-desc">
                    <template v-if="plugin.homepage">
                      {{ pluginDescriptionPrefix(plugin) }}
                      <a
                        :href="plugin.homepage"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click.stop
                        >{{ plugin.homepage }}</a
                      >
                    </template>
                    <template v-else>{{ plugin.description }}</template>
                  </p>
                </div>
              </div>

              <div class="plugin-card-bottom">
                <div class="plugin-status">
                  <template v-if="plugin.remote">
                    <ElTag type="warning" size="small" effect="light">未下载</ElTag>
                    <ElText type="info" size="small">来源：{{ plugin.source }}</ElText>
                  </template>
                  <template v-else>
                    <ElTag v-if="plugin.enabled" type="success" size="small" effect="light"
                      >已启用</ElTag
                    >
                    <ElTag v-else type="info" size="small" effect="plain">未启用</ElTag>
                    <ElText
                      :type="plugin.configured ? 'success' : 'warning'"
                      size="small"
                      class="config-status"
                    >
                      <ArtSvgIcon
                        :icon="
                          plugin.configured ? 'ri:checkbox-circle-line' : 'ri:error-warning-line'
                        "
                      />
                      {{ plugin.configured ? '已配置' : '未配置' }}
                    </ElText>
                  </template>
                </div>
                <div class="plugin-actions">
                  <template v-if="plugin.remote">
                    <ElButton
                      type="primary"
                      size="small"
                      :loading="downloadingId === plugin.id"
                      @click="handleDownload(plugin)"
                    >
                      下载
                    </ElButton>
                  </template>
                  <template v-else>
                    <ElButton
                      v-if="configTarget(plugin)"
                      size="small"
                      :disabled="!plugin.enabled"
                      @click="goConfig(plugin)"
                    >
                      配置
                    </ElButton>
                    <ElButton
                      :type="plugin.enabled ? 'default' : 'primary'"
                      size="small"
                      :loading="togglingId === plugin.id"
                      @click="handleToggle(plugin)"
                    >
                      {{ plugin.enabled ? '停用' : '启用' }}
                    </ElButton>
                  </template>
                </div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </ElCard>

    <ElDialog v-model="sourceDialogVisible" title="软件源管理" width="560px">
      <div class="source-add">
        <ElInput
          v-model="newSourceUrl"
          placeholder="软件源清单地址，如 https://example.com/index.json"
        />
        <ElInput
          v-model="newSourceName"
          placeholder="名称（可选，留空自动获取）"
          class="source-name-input"
        />
        <ElButton type="primary" :loading="addingSource" @click="handleAddSource">添加</ElButton>
      </div>
      <ElTable :data="sources" size="small" class="source-table">
        <ElTableColumn label="状态" width="70" align="center">
          <template #default="{ row }">
            <span class="source-dot" :class="`is-${row.state}`" />
          </template>
        </ElTableColumn>
        <ElTableColumn prop="name" label="名称" width="130" show-overflow-tooltip />
        <ElTableColumn prop="url" label="地址" show-overflow-tooltip />
        <ElTableColumn label="操作" width="90" align="center">
          <template #default="{ row }">
            <ElButton link type="danger" size="small" @click="handleDeleteSource(row)"
              >删除</ElButton
            >
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty description="暂无软件源" :image-size="60" />
        </template>
      </ElTable>
      <div class="source-tip"
        >软件源是一个远程插件仓库清单（JSON），添加后可浏览并下载其中的插件。</div
      >
    </ElDialog>

    <ElBacktop target="#app-main" :right="32" :bottom="32" />
  </div>
</template>

<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { useRouter } from 'vue-router'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { FolderAdd, Refresh, Search } from '@element-plus/icons-vue'
  import {
    fetchAddPluginSource,
    fetchDeletePluginSource,
    fetchDownloadPlugin,
    fetchPluginList,
    fetchTogglePlugin,
    PluginCategoryGroup,
    PluginInfo,
    PluginSource
  } from '@/api/system-manage'

  defineOptions({ name: 'PluginStore' })

  const router = useRouter()
  const loading = ref(false)
  const togglingId = ref('')
  const downloadingId = ref('')
  const categories = ref<PluginCategoryGroup[]>([])
  const sources = ref<PluginSource[]>([])

  const activeTab = ref('all')
  const searchText = ref('')
  let searchTimer: ReturnType<typeof setTimeout> | undefined

  const sourceDialogVisible = ref(false)
  const addingSource = ref(false)
  const newSourceUrl = ref('')
  const newSourceName = ref('')

  const visibleGroups = computed(() => {
    if (activeTab.value === 'all') return categories.value.filter((g) => g.plugins.length)
    return categories.value.filter((g) => g.category === activeTab.value && g.plugins.length)
  })

  const pluginDescriptionPrefix = (plugin: PluginInfo): string => {
    if (!plugin.homepage) return plugin.description
    return plugin.description.replace(plugin.homepage, '')
  }

  // 需要独立配置页的插件；其余按类别归到统一配置页
  const pluginConfigPaths: Record<string, string> = {
    epay: '/system/epay-config',
    'epay-v2': '/system/epay-config'
  }

  // 配置入口；返回空串表示无独立配置页。
  // 实名服务商共用系统配置的实名页签，新增服务商无需再改这里。
  const configTarget = (plugin: PluginInfo): string => {
    const mapped = pluginConfigPaths[plugin.id]
    if (mapped) return mapped
    if (plugin.category === 'realname') return '/system/config?tab=realname'
    return ''
  }

  const loadPlugins = async () => {
    loading.value = true
    try {
      const data = await fetchPluginList({ q: searchText.value || undefined })
      categories.value = data.categories || []
      sources.value = data.sources || []
    } catch {
      ElMessage.error('插件列表加载失败')
    } finally {
      loading.value = false
    }
  }

  const handleSearchInput = () => {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(loadPlugins, 400)
  }

  const handleToggle = async (plugin: PluginInfo) => {
    if (!plugin.enabled) {
      try {
        await ElMessageBox.confirm(
          `启用「${plugin.name}」后，同分区其他插件将自动停用，确认启用？`,
          '启用插件',
          { confirmButtonText: '启用', cancelButtonText: '取消', type: 'warning' }
        )
      } catch {
        return
      }
    }
    togglingId.value = plugin.id
    try {
      await fetchTogglePlugin(plugin.id, !plugin.enabled)
      ElMessage.success(plugin.enabled ? '插件已停用' : `已启用「${plugin.name}」`)
      await loadPlugins()
    } catch {
      ElMessage.error('操作失败')
    } finally {
      togglingId.value = ''
    }
  }

  const handleDownload = async (plugin: PluginInfo) => {
    downloadingId.value = plugin.id
    try {
      await fetchDownloadPlugin(plugin.id)
      ElMessage.success(`「${plugin.name}」已下载到本地`)
      await loadPlugins()
    } catch {
      ElMessage.error('插件下载失败')
    } finally {
      downloadingId.value = ''
    }
  }

  const handleAddSource = async () => {
    const url = newSourceUrl.value.trim()
    if (!url) {
      ElMessage.warning('请输入软件源清单地址')
      return
    }
    addingSource.value = true
    try {
      await fetchAddPluginSource(newSourceName.value.trim(), url)
      ElMessage.success('软件源已添加')
      newSourceUrl.value = ''
      newSourceName.value = ''
      await loadPlugins()
    } catch (e: any) {
      ElMessage.error(e?.message || '软件源添加失败，请检查清单地址')
    } finally {
      addingSource.value = false
    }
  }

  const handleDeleteSource = async (row: PluginSource) => {
    try {
      await ElMessageBox.confirm(
        `确认删除软件源「${row.name}」？已下载到本地的插件不受影响。`,
        '删除软件源',
        {
          confirmButtonText: '删除',
          cancelButtonText: '取消',
          type: 'warning'
        }
      )
    } catch {
      return
    }
    try {
      await fetchDeletePluginSource(row.id)
      ElMessage.success('软件源已删除')
      await loadPlugins()
    } catch {
      ElMessage.error('删除失败')
    }
  }

  const goConfig = (plugin: PluginInfo) => {
    const target = configTarget(plugin)
    if (target) router.push(target)
  }

  onMounted(loadPlugins)
</script>

<style lang="scss" scoped>
  .plugin-store {
    .store-header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      margin-bottom: 8px;

      .store-title {
        margin: 0;
        font-size: 20px;
        color: var(--art-gray-900);
      }

      .store-subtitle {
        margin: 6px 0 0;
        font-size: 13px;
        color: var(--art-gray-600);
      }

      .store-header-actions {
        display: flex;
        gap: 10px;
        align-items: center;
      }
    }

    .store-toolbar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 16px;

      .store-tabs {
        flex: 1;
        min-width: 0;

        :deep(.el-tabs__header) {
          margin-bottom: 0;
        }
      }

      .store-search {
        width: 260px;
        flex-shrink: 0;
      }
    }

    .store-body {
      min-height: 200px;
    }

    .plugin-section {
      margin-bottom: 28px;

      &:last-child {
        margin-bottom: 0;
      }

      .section-header {
        display: flex;
        align-items: center;
        gap: 10px;
        padding-bottom: 12px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--art-border-color);

        strong {
          font-size: 15px;
          color: var(--art-gray-900);
        }
      }
    }

    .plugin-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
      gap: 16px;
    }

    .plugin-card {
      display: flex;
      flex-direction: column;
      justify-content: space-between;
      padding: 18px;
      border: 1px solid var(--art-border-color);
      border-radius: 8px;
      transition:
        border-color 0.15s,
        box-shadow 0.15s;

      &:hover {
        box-shadow: 0 4px 16px rgb(0 0 0 / 6%);
      }

      &.is-enabled {
        border-color: var(--el-color-primary-light-5);
      }

      .plugin-card-top {
        display: flex;
        gap: 14px;
      }

      .plugin-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 46px;
        height: 46px;
        font-size: 24px;
        color: var(--el-color-primary);
        background: var(--el-color-primary-light-9);
        border-radius: 10px;
        flex-shrink: 0;
      }

      .plugin-meta {
        flex: 1;
        min-width: 0;

        .plugin-name {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;

          strong {
            font-size: 14px;
            color: var(--art-gray-900);
          }
        }

        .plugin-desc {
          margin: 6px 0 0;
          font-size: 12px;
          line-height: 1.6;
          color: var(--art-gray-600);

          a {
            color: var(--el-color-primary);
            text-decoration: none;
            overflow-wrap: anywhere;

            &:hover {
              text-decoration: underline;
            }
          }
        }
      }

      .plugin-card-bottom {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding-top: 14px;
        margin-top: 14px;
        border-top: 1px dashed var(--art-border-color);

        .plugin-status {
          display: flex;
          align-items: center;
          gap: 8px;

          .config-status {
            display: inline-flex;
            align-items: center;
            gap: 3px;
          }
        }

        .plugin-actions {
          display: flex;
          gap: 8px;

          :deep(.el-button.el-button--small) {
            min-width: 64px;
            height: 24px !important;
            padding: 5px 12px;
            margin-left: 0;
          }
        }
      }
    }

    .source-add {
      display: flex;
      gap: 10px;
      margin-bottom: 16px;

      .source-name-input {
        width: 180px;
        flex-shrink: 0;
      }
    }

    .source-table {
      margin-bottom: 12px;
    }

    .source-dot {
      display: inline-block;
      width: 8px;
      height: 8px;
      border-radius: 50%;

      &.is-ok {
        background: var(--el-color-success);
      }

      &.is-error {
        background: var(--el-color-danger);
      }

      &.is-unknown {
        background: var(--el-color-info);
      }
    }

    .source-tip {
      font-size: 12px;
      color: var(--art-gray-600);
    }

    @media (max-width: 768px) {
      .store-toolbar {
        flex-direction: column;
        align-items: stretch;

        .store-search {
          width: 100%;
        }
      }

      .source-add {
        flex-direction: column;

        .source-name-input {
          width: 100%;
        }
      }
    }
  }
</style>
