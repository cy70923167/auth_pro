<template>
  <div class="system-config-page">
    <ElCard v-loading="loading" class="config-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <h2>系统设置</h2>
            <p>按分区管理网站信息、公告和用户端功能开关</p>
          </div>
          <div class="header-actions">
            <ElButton
              :disabled="saving || realnameSaving || featureSwitchSaving"
              @click="handleReset"
            >
              重置
            </ElButton>
            <ElButton
              type="primary"
              :loading="saving || realnameSaving"
              :disabled="featureSwitchSaving"
              @click="handleSave"
            >
              保存配置
            </ElButton>
          </div>
        </div>
      </template>

      <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" class="config-form">
        <ElTabs v-model="activeTab" class="config-tabs">
          <ElTabPane label="网站信息" name="site">
            <ElRow :gutter="24">
              <ElCol :xs="24" :lg="16">
                <section class="section-card">
                  <div class="section-title">
                    <div>
                      <strong>网站信息</strong>
                      <span>用于后台、登录页和用户端的统一展示</span>
                    </div>
                  </div>

                  <ElRow :gutter="16">
                    <ElCol :xs="24" :md="12">
                      <ElFormItem label="网站名称" prop="siteName">
                        <ElInput
                          v-model.trim="form.siteName"
                          maxlength="60"
                          show-word-limit
                          placeholder="请输入网站名称"
                        />
                      </ElFormItem>
                    </ElCol>
                    <ElCol :xs="24" :md="12">
                      <ElFormItem label="网站副标题" prop="siteSubtitle">
                        <ElInput
                          v-model.trim="form.siteSubtitle"
                          maxlength="120"
                          show-word-limit
                          placeholder="请输入网站副标题"
                        />
                      </ElFormItem>
                    </ElCol>
                    <ElCol :xs="24" :md="12">
                      <ElFormItem label="建站时间（系统安装时间）">
                        <ElInput v-model="form.installedAt" disabled />
                      </ElFormItem>
                    </ElCol>
                    <ElCol :xs="24" :md="12">
                      <ElFormItem label="站长 QQ" prop="stationQQ">
                        <ElInput
                          v-model.trim="form.stationQQ"
                          maxlength="30"
                          placeholder="请输入站长 QQ"
                        />
                      </ElFormItem>
                    </ElCol>
                    <ElCol :xs="24">
                      <ElFormItem label="网站备案号" prop="icpNumber">
                        <ElInput
                          v-model.trim="form.icpNumber"
                          maxlength="100"
                          show-word-limit
                          placeholder="例如：京ICP备xxxxxxxx号"
                        />
                      </ElFormItem>
                    </ElCol>
                  </ElRow>

                  <ElFormItem label="网站 Logo" prop="siteLogo">
                    <div class="logo-uploader-wrap">
                      <ElUpload
                        accept="image/png,image/jpeg,image/webp"
                        :show-file-list="false"
                        :before-upload="handleLogoUpload"
                      >
                        <div class="logo-uploader">
                          <img :src="previewLogo" alt="logo" />
                          <div class="upload-mask">
                            <ArtSvgIcon icon="ri:upload-cloud-2-line" />
                            <span>上传 Logo</span>
                          </div>
                        </div>
                      </ElUpload>

                      <div class="logo-actions">
                        <ElText type="info">支持 PNG、JPG、WebP，大小不超过 512KB</ElText>
                        <div class="action-buttons">
                          <ElButton @click="resetLogo">恢复已保存</ElButton>
                          <ElButton type="danger" plain @click="removeLogo">移除 Logo</ElButton>
                        </div>
                      </div>
                    </div>
                  </ElFormItem>
                </section>
              </ElCol>

              <ElCol :xs="24" :lg="8">
                <div class="preview-panel">
                  <div class="preview-title">配置预览</div>
                  <div class="preview-card">
                    <img :src="previewLogo" alt="logo preview" />
                    <div>
                      <strong>{{ form.siteName || '网站名称' }}</strong>
                      <span>{{ form.siteSubtitle || '网站副标题' }}</span>
                    </div>
                  </div>
                  <ElDescriptions :column="1" border>
                    <ElDescriptionsItem label="建站时间">
                      {{ form.installedAt || '-' }}
                    </ElDescriptionsItem>
                    <ElDescriptionsItem label="站长 QQ">{{
                      form.stationQQ || '-'
                    }}</ElDescriptionsItem>
                    <ElDescriptionsItem label="备案号">{{
                      form.icpNumber || '-'
                    }}</ElDescriptionsItem>
                    <ElDescriptionsItem label="普通用户注册">
                      <ElTag :type="form.registrationEnabled ? 'success' : 'info'">
                        {{ form.registrationEnabled ? '允许' : '已关闭' }}
                      </ElTag>
                    </ElDescriptionsItem>
                    <ElDescriptionsItem label="用户自助购买">
                      <ElTag :type="form.selfPurchaseEnabled ? 'success' : 'info'">
                        {{ form.selfPurchaseEnabled ? '允许' : '已关闭' }}
                      </ElTag>
                    </ElDescriptionsItem>
                  </ElDescriptions>
                  <div v-if="form.domainLicenseNotice" class="notice-preview">
                    <strong>网站公告</strong>
                    <p>{{ form.domainLicenseNotice }}</p>
                  </div>
                </div>
              </ElCol>
            </ElRow>
          </ElTabPane>

          <ElTabPane label="网站公告" name="notice">
            <section class="section-card">
              <div class="section-title">
                <div>
                  <strong>网站公告</strong>
                  <span>展示域名授权相关说明、服务时间或联系方式</span>
                </div>
              </div>
              <ElRow :gutter="24">
                <ElCol :xs="24" :lg="16">
                  <ElFormItem label="域名授权网站公告" prop="domainLicenseNotice">
                    <ElInput
                      v-model="form.domainLicenseNotice"
                      type="textarea"
                      :rows="10"
                      maxlength="2000"
                      show-word-limit
                      placeholder="请输入域名授权网站公告"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :lg="8">
                  <div class="notice-preview-panel">
                    <div class="preview-title">公告预览</div>
                    <div v-if="form.domainLicenseNotice" class="notice-preview notice-preview-full">
                      <p>{{ form.domainLicenseNotice }}</p>
                    </div>
                    <ElText v-else type="info">暂无公告内容</ElText>
                  </div>
                </ElCol>
              </ElRow>
            </section>
          </ElTabPane>

          <ElTabPane label="用户端功能" name="features">
            <section class="section-card">
              <div class="section-title">
                <div>
                  <strong>用户端功能</strong>
                  <span>开关切换后自动保存；关闭后对应服务端功能立即停止</span>
                </div>
              </div>
              <div class="switch-list">
                <div class="switch-item">
                  <div>
                    <strong>普通用户注册</strong>
                    <span>是否允许访客注册普通用户账号</span>
                  </div>
                  <ElSwitch
                    v-model="form.registrationEnabled"
                    inline-prompt
                    active-text="允许"
                    inactive-text="关闭"
                    :loading="featureSwitchSavingState.registrationEnabled"
                    :disabled="saving || featureSwitchSaving"
                    @change="(value) => handleFeatureSwitchChange('registrationEnabled', value)"
                  />
                </div>
                <div class="switch-item">
                  <div>
                    <strong>用户自助购买</strong>
                    <span>是否允许普通用户自行选择套餐并购买授权</span>
                  </div>
                  <ElSwitch
                    v-model="form.selfPurchaseEnabled"
                    inline-prompt
                    active-text="允许"
                    inactive-text="关闭"
                    :loading="featureSwitchSavingState.selfPurchaseEnabled"
                    :disabled="saving || featureSwitchSaving"
                    @change="(value) => handleFeatureSwitchChange('selfPurchaseEnabled', value)"
                  />
                </div>
                <div class="switch-item">
                  <div>
                    <strong>盗版入库</strong>
                    <span>开启后自动检测未授权域名或 IP 并写入盗版记录</span>
                  </div>
                  <ElSwitch
                    v-model="form.piracyDetectionEnabled"
                    inline-prompt
                    active-text="开启"
                    inactive-text="关闭"
                    :loading="featureSwitchSavingState.piracyDetectionEnabled"
                    :disabled="saving || featureSwitchSaving"
                    @change="(value) => handleFeatureSwitchChange('piracyDetectionEnabled', value)"
                  />
                </div>
              </div>
            </section>
          </ElTabPane>

          <ElTabPane label="实名认证" name="realname">
            <section class="section-card">
              <div class="section-title">
                <div>
                  <strong>实名认证</strong>
                  <span>支持支付宝、快瞳和靓仔聚合认证，按应用控制是否强制用户实名</span>
                </div>
                <ElTag :type="realnameForm.enabled ? 'success' : 'info'" effect="light">
                  {{ realnameForm.enabled ? '已启用' : '未启用' }}
                </ElTag>
              </div>

              <!-- 启用开关 -->
              <div class="realname-master-switch" :class="{ 'is-enabled': realnameForm.enabled }">
                <div class="switch-content">
                  <div class="switch-icon">
                    <ArtSvgIcon icon="ri:shield-user-line" />
                  </div>
                  <div>
                    <strong>启用实名认证</strong>
                    <span>开启后，勾选了「要求实名」的应用将拦截未实名用户的授权安装</span>
                  </div>
                </div>
                <ElSwitch
                  v-model="realnameForm.enabled"
                  inline-prompt
                  active-text="开启"
                  inactive-text="关闭"
                  size="large"
                />
              </div>

              <ElRow :gutter="24" class="realname-body">
                <!-- 左：接口配置 -->
                <ElCol :xs="24" :lg="14">
                  <div class="config-block">
                    <div class="block-header">
                      <span class="block-num">1</span>
                      <div>
                        <strong>服务商</strong>
                        <span>由应用商店启用的实名认证插件决定</span>
                      </div>
                      <ElButton
                        size="small"
                        class="block-link-btn"
                        @click="router.push('/plugin-store')"
                      >
                        前往应用商店
                      </ElButton>
                    </div>

                    <ElAlert
                      v-if="!realnameConfig.pluginEnabled"
                      type="warning"
                      :closable="false"
                      title="尚未启用实名认证服务商插件"
                      description="请前往应用商店，在「实名认证服务商」分区启用一个插件后再回来配置"
                      class="plugin-off-alert"
                    />
                    <template v-else>
                      <div class="provider-item is-static is-active">
                        <ArtSvgIcon
                          :icon="realnameProviderIcon(realnameForm.provider)"
                          class="provider-icon"
                          :class="realnameForm.provider"
                        />
                        <div class="provider-info">
                          <strong>{{ realnameProviderName(realnameForm.provider) }}</strong>
                          <span>{{ realnameProviderDescription(realnameForm.provider) }}</span>
                        </div>
                        <ElTag type="success" size="small" effect="light">当前服务商</ElTag>
                      </div>
                      <div class="field-hint">
                        <ArtSvgIcon icon="ri:information-line" />
                        <span>如需更换服务商，请到应用商店停用当前插件并启用目标插件</span>
                      </div>
                    </template>
                  </div>

                  <div
                    v-if="realnameConfig.pluginEnabled && realnameForm.provider === 'kuaitong'"
                    class="config-block"
                  >
                    <div class="block-header">
                      <span class="block-num">2</span>
                      <div>
                        <strong>快瞳凭证</strong>
                        <span>快瞳开放平台 AI 接口的访问凭证</span>
                      </div>
                    </div>

                    <ElFormItem label="认证类型">
                      <ElSelect
                        v-model="realnameForm.kuaitongAuthType"
                        placeholder="请选择认证类型"
                        style="width: 100%"
                      >
                        <ElOption label="二要素认证" value="two_element" />
                        <ElOption label="人脸认证" value="face" />
                      </ElSelect>
                      <div class="field-hint">
                        <ArtSvgIcon icon="ri:information-line" />
                        <span v-if="realnameForm.kuaitongAuthType === 'two_element'">
                          提交姓名和身份证号后直接核验，无需扫码
                        </span>
                        <span v-else>提交后生成二维码，扫码拍照完成人脸核验</span>
                      </div>
                    </ElFormItem>

                    <ElFormItem label="accessKey">
                      <ElInput
                        v-model.trim="realnameForm.kuaitongAccessKey"
                        maxlength="64"
                        placeholder="快瞳平台分配的 accessKey"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:key-2-line" class="input-icon" />
                        </template>
                      </ElInput>
                    </ElFormItem>

                    <ElFormItem>
                      <template #label>
                        <span>accessSecret</span>
                        <ElTag
                          v-if="realnameConfig.kuaitongSecretSet"
                          type="success"
                          size="small"
                          effect="plain"
                          class="label-tag"
                          >已配置</ElTag
                        >
                      </template>
                      <ElInput
                        v-model="realnameForm.kuaitongSecret"
                        type="password"
                        show-password
                        maxlength="64"
                        :placeholder="
                          realnameConfig.kuaitongSecretSet
                            ? '已保存密钥，留空则不修改；输入新密钥则覆盖'
                            : '快瞳平台分配的 accessSecret'
                        "
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:lock-2-line" class="input-icon" />
                        </template>
                      </ElInput>
                      <div class="field-hint">
                        <ArtSvgIcon icon="ri:information-line" />
                        <span>鉴权 token 由服务端自动缓存与刷新，无需手动管理</span>
                      </div>
                    </ElFormItem>
                  </div>

                  <div
                    v-else-if="realnameConfig.pluginEnabled && realnameForm.provider === 'tencent'"
                    class="config-block"
                  >
                    <div class="block-header">
                      <span class="block-num">2</span>
                      <div>
                        <strong>靓仔聚合认证凭证</strong>
                        <span>聚合实名认证开放 API 的 HMAC 鉴权凭证</span>
                      </div>
                      <ElButton
                        type="primary"
                        link
                        class="block-link-btn"
                        @click="openTencentRealnameDocs"
                      >
                        接入文档
                      </ElButton>
                    </div>

                    <ElFormItem label="API Key">
                      <ElInput
                        v-model.trim="realnameForm.tencentApiKey"
                        maxlength="128"
                        placeholder="开放平台分配的 API Key"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:key-2-line" class="input-icon" />
                        </template>
                      </ElInput>
                    </ElFormItem>

                    <ElFormItem>
                      <template #label>
                        <span>API Secret</span>
                        <ElTag
                          v-if="realnameConfig.tencentSecretSet"
                          type="success"
                          size="small"
                          effect="plain"
                          class="label-tag"
                          >已配置</ElTag
                        >
                      </template>
                      <ElInput
                        v-model="realnameForm.tencentApiSecret"
                        type="password"
                        show-password
                        maxlength="128"
                        :placeholder="
                          realnameConfig.tencentSecretSet
                            ? '已保存密钥，留空则不修改；输入新密钥则覆盖'
                            : '开放平台分配的 API Secret'
                        "
                      />
                    </ElFormItem>

                    <ElFormItem label="接口地址">
                      <ElInput
                        v-model.trim="realnameForm.tencentBaseUrl"
                        maxlength="500"
                        placeholder="https://real.4775.cn/common/openapi"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:links-line" class="input-icon" />
                        </template>
                      </ElInput>
                    </ElFormItem>

                    <ElFormItem label="产品编码">
                      <ElInput model-value="cloud_tencent_renlian_zq" disabled>
                        <template #prefix>
                          <ArtSvgIcon icon="ri:shield-check-line" class="input-icon" />
                        </template>
                      </ElInput>
                      <div class="field-hint">
                        <ArtSvgIcon icon="ri:information-line" />
                        <span>固定使用腾讯云增强人脸认证，避免误选异步认证产品</span>
                      </div>
                    </ElFormItem>

                    <ElFormItem label="扣费方式">
                      <ElSwitch
                        v-model="realnameForm.tencentUsePackage"
                        active-text="套餐优先"
                        inactive-text="余额扣费"
                      />
                      <div class="field-hint">
                        <ArtSvgIcon icon="ri:information-line" />
                        <span>开启后自动匹配可用套餐；关闭后直接从账户余额扣费</span>
                      </div>
                    </ElFormItem>
                  </div>

                  <div
                    v-else-if="realnameConfig.pluginEnabled && realnameForm.provider === 'xiaomu'"
                    class="config-block"
                  >
                    <div class="block-header">
                      <span class="block-num">2</span>
                      <div>
                        <strong>小沐聚合实名凭证</strong>
                        <span>聚合实名认证开放 API 的 AppKey / AppSecret 请求头鉴权</span>
                      </div>
                    </div>

                    <ElFormItem label="AppKey">
                      <ElInput
                        v-model.trim="realnameForm.xiaomuAppKey"
                        maxlength="128"
                        placeholder="以 ak_ 开头的应用标识"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:key-2-line" class="input-icon" />
                        </template>
                      </ElInput>
                    </ElFormItem>

                    <ElFormItem>
                      <template #label>
                        <span>AppSecret</span>
                        <ElTag
                          v-if="realnameConfig.xiaomuSecretSet"
                          type="success"
                          size="small"
                          effect="plain"
                          class="label-tag"
                          >已配置</ElTag
                        >
                      </template>
                      <ElInput
                        v-model="realnameForm.xiaomuAppSecret"
                        type="password"
                        show-password
                        maxlength="128"
                        :placeholder="
                          realnameConfig.xiaomuSecretSet
                            ? '已保存密钥，留空则不修改；输入新密钥则覆盖'
                            : '以 as_ 开头的应用密钥'
                        "
                      />
                    </ElFormItem>

                    <ElFormItem label="接口地址">
                      <ElInput
                        v-model.trim="realnameForm.xiaomuBaseUrl"
                        maxlength="500"
                        placeholder="https://smapi.x1m1.cn"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:links-line" class="input-icon" />
                        </template>
                      </ElInput>
                    </ElFormItem>

                    <ElFormItem label="认证产品">
                      <ElSelect
                        v-model="realnameForm.xiaomuProductMode"
                        placeholder="选择唯一启用的认证产品"
                        style="width: 260px"
                      >
                        <ElOption label="三要素核验" value="three_element" />
                        <ElOption label="人脸认证" value="face_h5" />
                        <ElOption label="微信实名" value="tencent_h5" />
                      </ElSelect>
                      <div class="field-hint">
                        <ArtSvgIcon icon="ri:information-line" />
                        <span v-if="realnameForm.xiaomuProductMode === 'three_element'">
                          姓名 + 身份证 + 手机号一致性核验，用户认证时需临时填写本人手机号
                        </span>
                        <span v-else-if="realnameForm.xiaomuProductMode === 'face_h5'">
                          用户跳转小沐活体认证页完成拍照，与人脸权威库比对
                        </span>
                        <span v-else>用户跳转微信 / 腾讯官方实名页完成认证</span>
                      </div>
                    </ElFormItem>
                  </div>

                  <div v-else-if="realnameConfig.pluginEnabled" class="config-block">
                    <div class="block-header">
                      <span class="block-num">2</span>
                      <div>
                        <strong>支付宝凭证</strong>
                        <span>支付宝开放平台应用的密钥信息</span>
                      </div>
                    </div>

                    <ElFormItem label="支付宝应用 AppID">
                      <ElInput
                        v-model.trim="realnameForm.appId"
                        maxlength="32"
                        placeholder="开放平台应用的 AppID"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:apps-2-line" class="input-icon" />
                        </template>
                      </ElInput>
                    </ElFormItem>

                    <ElFormItem>
                      <template #label>
                        <span>应用私钥（RSA2）</span>
                        <ElTag
                          v-if="realnameConfig.privateKeySet"
                          type="success"
                          size="small"
                          effect="plain"
                          class="label-tag"
                          >已配置</ElTag
                        >
                      </template>
                      <ElInput
                        v-model="realnameForm.privateKey"
                        type="textarea"
                        :rows="4"
                        :placeholder="
                          realnameConfig.privateKeySet
                            ? '已保存私钥，留空则不修改；粘贴新私钥则覆盖'
                            : '粘贴 PKCS#1 / PKCS#8 私钥，支持带或不带 PEM 头尾'
                        "
                      />
                    </ElFormItem>

                    <ElFormItem label="支付宝公钥">
                      <ElInput
                        v-model.trim="realnameForm.alipayPublicKey"
                        type="textarea"
                        :rows="3"
                        placeholder="支付宝开放平台提供的支付宝公钥"
                      />
                    </ElFormItem>

                    <ElFormItem label="网关地址">
                      <ElInput
                        v-model.trim="realnameForm.gateway"
                        placeholder="https://openapi.alipay.com/gateway.do"
                      >
                        <template #prefix>
                          <ArtSvgIcon icon="ri:global-line" class="input-icon" />
                        </template>
                      </ElInput>
                      <div class="field-hint">
                        <span>沙箱调试：</span>
                        <ElButton
                          type="primary"
                          link
                          size="small"
                          @click="
                            realnameForm.gateway =
                              'https://openapi-sandbox.dl.alipaydev.com/gateway.do'
                          "
                          >填入沙箱网关</ElButton
                        >
                        <ElButton
                          type="primary"
                          link
                          size="small"
                          @click="realnameForm.gateway = 'https://openapi.alipay.com/gateway.do'"
                          >恢复正式网关</ElButton
                        >
                      </div>
                    </ElFormItem>
                  </div>
                </ElCol>

                <!-- 右：接入说明 + 应用范围 -->
                <ElCol :xs="24" :lg="10">
                  <div class="config-block help-block">
                    <div class="block-header">
                      <ArtSvgIcon icon="ri:information-line" class="help-icon" />
                      <div>
                        <strong>接入说明</strong>
                        <span>开通服务与用户使用流程</span>
                      </div>
                    </div>
                    <ul class="help-list">
                      <template v-if="realnameForm.provider === 'alipay'">
                        <li>需要在支付宝开放平台开通「金融级实人认证」</li>
                        <li>用户在「个人信息」页输入姓名 + 身份证号，扫码刷脸完成认证</li>
                      </template>
                      <template v-else-if="realnameForm.provider === 'kuaitong'">
                        <li>需要在快瞳平台开通「身份证姓名号码核验」接口权限</li>
                        <li>用户输入姓名 + 身份证号后扫码拍照，完成人证合一核验</li>
                      </template>
                      <template v-else-if="realnameForm.provider === 'xiaomu'">
                        <li>需要在小沐聚合实名平台开通对应认证产品并获取 AppKey / AppSecret</li>
                        <li v-if="realnameForm.xiaomuProductMode === 'three_element'">
                          用户输入姓名 + 身份证号 + 本人手机号，提交后直接返回核验结果
                        </li>
                        <li v-else>用户输入姓名 + 身份证号后跳转上游认证页，完成后自动回写结果</li>
                      </template>
                      <template v-else>
                        <li>需要在聚合实名认证平台开通「腾讯云增强人脸」产品</li>
                        <li>用户输入姓名 + 身份证号后扫码拍照，服务端提交人脸 Base64 完成核验</li>
                      </template>
                      <li>勾选应用对未实名用户返回 <code>realname_required</code>，禁止安装</li>
                      <li>未勾选的应用不受实名限制，可正常安装使用</li>
                    </ul>
                  </div>

                  <div class="config-block">
                    <div class="block-header">
                      <span class="block-num">3</span>
                      <div>
                        <strong>实名应用范围</strong>
                        <span>勾选的应用将强制要求用户完成实名</span>
                      </div>
                      <ElText class="block-count" type="info" size="small">
                        已选 {{ realnameForm.requireAppIds.length }} /
                        {{ realnameConfig.apps.length }}
                      </ElText>
                    </div>

                    <div v-if="realnameConfig.apps.length" class="app-select-list">
                      <div
                        v-for="app in realnameConfig.apps"
                        :key="app.id"
                        class="app-select-item"
                        :class="{ 'is-checked': realnameForm.requireAppIds.includes(app.id) }"
                        @click="toggleRequireApp(app.id)"
                      >
                        <ElCheckbox
                          :model-value="realnameForm.requireAppIds.includes(app.id)"
                          @click.stop
                          @change="() => toggleRequireApp(app.id)"
                        />
                        <div class="app-info">
                          <strong>{{ app.appName }}</strong>
                          <span>{{ app.appKey }}</span>
                        </div>
                        <ElTag
                          v-if="realnameForm.requireAppIds.includes(app.id)"
                          type="warning"
                          size="small"
                          effect="light"
                          >需实名</ElTag
                        >
                      </div>
                    </div>
                    <div v-else class="app-empty-state">
                      <ArtSvgIcon icon="ri:inbox-line" />
                      <p>暂无应用</p>
                      <ElText type="info" size="small">请先在应用管理中创建应用</ElText>
                    </div>
                  </div>
                </ElCol>
              </ElRow>
            </section>

            <section class="section-card realname-records">
              <div class="section-title">
                <div>
                  <strong>认证记录</strong>
                  <span>展示全部认证结果，可按结果、服务商和主体信息筛选，敏感信息已脱敏</span>
                </div>
                <ElButton size="small" @click="loadRealnameRecords">刷新</ElButton>
              </div>

              <div class="record-filters">
                <ElInput
                  v-model="recordQuery.keyword"
                  placeholder="搜索主体 / 邮箱 / 姓名 / 身份证号"
                  clearable
                  style="width: 220px"
                  @keyup.enter="handleRecordSearch"
                />
                <ElSelect
                  v-model="recordQuery.provider"
                  placeholder="服务商"
                  clearable
                  style="width: 130px"
                >
                  <ElOption label="支付宝" value="alipay" />
                  <ElOption label="快瞳" value="kuaitong" />
                  <ElOption label="靓仔聚合认证" value="tencent" />
                  <ElOption label="小沐聚合实名" value="xiaomu" />
                </ElSelect>
                <ElSelect
                  v-model="recordQuery.status"
                  placeholder="认证结果"
                  clearable
                  style="width: 130px"
                >
                  <ElOption label="认证成功" value="passed" />
                  <ElOption label="认证失败" value="failed" />
                </ElSelect>
                <ElButton type="primary" @click="handleRecordSearch">查询</ElButton>
              </div>

              <ElTable v-loading="recordLoading" :data="recordList" size="small" stripe>
                <ElTableColumn label="主体" min-width="170">
                  <template #default="{ row }">
                    <div class="record-owner">
                      <ElTag
                        size="small"
                        :type="row.ownerType === 'agent' ? 'warning' : 'primary'"
                        effect="plain"
                      >
                        {{ row.ownerType === 'agent' ? '代理' : '用户' }}
                      </ElTag>
                      <div>
                        <div>{{ row.ownerName || '-' }}</div>
                        <span class="record-email">{{ row.ownerEmail }}</span>
                      </div>
                    </div>
                  </template>
                </ElTableColumn>
                <ElTableColumn prop="realName" label="姓名" width="90" />
                <ElTableColumn prop="idCard" label="身份证号" min-width="150" />
                <ElTableColumn label="服务商" width="90">
                  <template #default="{ row }">
                    {{ realnameProviderName(row.provider) }}
                  </template>
                </ElTableColumn>
                <ElTableColumn label="结果" width="100">
                  <template #default="{ row }">
                    <ElTag
                      :type="row.status === 'passed' ? 'success' : 'danger'"
                      size="small"
                      effect="light"
                    >
                      {{ row.status === 'passed' ? '认证成功' : '认证失败' }}
                    </ElTag>
                  </template>
                </ElTableColumn>
                <ElTableColumn label="详细失败原因" min-width="420">
                  <template #default="{ row }">
                    <div v-if="row.status === 'failed'" class="record-fail-reason">
                      {{ row.failReason || '-' }}
                    </div>
                    <span v-else>-</span>
                  </template>
                </ElTableColumn>
                <ElTableColumn label="相似度" width="80">
                  <template #default="{ row }">{{ row.score ? row.score + '分' : '-' }}</template>
                </ElTableColumn>
                <ElTableColumn prop="serialNo" label="流水号" min-width="150" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.serialNo || '-' }}</template>
                </ElTableColumn>
                <ElTableColumn prop="createdAt" label="认证时间" width="150" />
                <template #empty>暂无认证记录</template>
              </ElTable>

              <div class="record-pagination">
                <ElPagination
                  v-model:current-page="recordQuery.page"
                  v-model:page-size="recordQuery.pageSize"
                  :total="recordTotal"
                  :page-sizes="[10, 20, 50]"
                  layout="total, sizes, prev, pager, next"
                  @change="loadRealnameRecords"
                />
              </div>
            </section>
          </ElTabPane>
        </ElTabs>
      </ElForm>
    </ElCard>
    <ElBacktop target="#app-main" :right="32" :bottom="32" />
  </div>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import { ElMessage } from 'element-plus'
  import {
    fetchSystemConfig,
    fetchUpdateSystemConfig,
    fetchUpdateSystemFeatureSwitch,
    fetchRealnameConfig,
    fetchUpdateRealnameConfig,
    fetchRealnameRecords
  } from '@/api/system-manage'
  import type {
    KuaitongAuthType,
    RealnameProvider,
    RealnameRecordItem,
    XiaomuProductMode
  } from '@/api/system-manage'
  import type {
    SystemConfigData,
    SystemFeatureSwitchKey,
    RealnameConfigData
  } from '@/api/system-manage'
  import { useSystemConfigStore } from '@/store/modules/system-config'
  import { useRoute, useRouter } from 'vue-router'

  defineOptions({ name: 'SystemConfig' })

  const MAX_LOGO_SIZE = 512 * 1024
  const ACCEPTED_LOGO_TYPES = ['image/png', 'image/jpeg', 'image/webp']

  const formRef = ref<FormInstance>()
  const loading = ref(false)
  const saving = ref(false)
  const route = useRoute()
  const router = useRouter()
  const activeTab = ref(typeof route.query.tab === 'string' ? route.query.tab : 'site')
  const realnameConfig = reactive<RealnameConfigData>({
    enabled: false,
    pluginEnabled: true,
    provider: 'alipay',
    appId: '',
    privateKeySet: false,
    alipayPublicKey: '',
    gateway: 'https://openapi.alipay.com/gateway.do',
    kuaitongAccessKey: '',
    kuaitongSecretSet: false,
    kuaitongAuthType: 'face',
    tencentApiKey: '',
    tencentSecretSet: false,
    tencentBaseUrl: 'https://real.4775.cn/common/openapi',
    tencentUsePackage: true,
    tencentProductCode: 'cloud_tencent_renlian_zq',
    xiaomuAppKey: '',
    xiaomuSecretSet: false,
    xiaomuBaseUrl: 'https://smapi.x1m1.cn',
    xiaomuProductMode: 'three_element',
    requireAppIds: [],
    apps: []
  })
  const realnameForm = reactive({
    enabled: false,
    provider: 'alipay' as RealnameProvider,
    appId: '',
    privateKey: '',
    alipayPublicKey: '',
    gateway: 'https://openapi.alipay.com/gateway.do',
    kuaitongAccessKey: '',
    kuaitongSecret: '',
    kuaitongAuthType: 'face' as KuaitongAuthType,
    tencentApiKey: '',
    tencentApiSecret: '',
    tencentBaseUrl: 'https://real.4775.cn/common/openapi',
    tencentUsePackage: true,
    xiaomuAppKey: '',
    xiaomuAppSecret: '',
    xiaomuBaseUrl: 'https://smapi.x1m1.cn',
    xiaomuProductMode: 'three_element' as XiaomuProductMode,
    requireAppIds: [] as number[]
  })
  const realnameSaving = ref(false)
  const featureSwitchSavingState = reactive<Record<SystemFeatureSwitchKey, boolean>>({
    registrationEnabled: false,
    selfPurchaseEnabled: false,
    piracyDetectionEnabled: false
  })
  const featureSwitchSaving = computed(() => Object.values(featureSwitchSavingState).some(Boolean))
  const systemConfigStore = useSystemConfigStore()
  const form = reactive<SystemConfigData>({
    siteName: '',
    siteSubtitle: '',
    siteLogo: '',
    installedAt: '',
    stationQQ: '',
    icpNumber: '',
    domainLicenseNotice: '',
    registrationEnabled: true,
    selfPurchaseEnabled: true,
    piracyDetectionEnabled: false
  })

  const savedConfig = ref<SystemConfigData>({
    siteName: '',
    siteSubtitle: '',
    siteLogo: '',
    installedAt: '',
    stationQQ: '',
    icpNumber: '',
    domainLicenseNotice: '',
    registrationEnabled: true,
    selfPurchaseEnabled: true,
    piracyDetectionEnabled: false
  })

  const previewLogo = computed(() => form.siteLogo || systemConfigStore.resolvedLogo)

  const rules: FormRules<SystemConfigData> = {
    siteName: [
      { required: true, message: '请输入网站名称', trigger: 'blur' },
      { min: 1, max: 60, message: '网站名称不能超过 60 个字符', trigger: 'blur' }
    ],
    siteSubtitle: [{ max: 120, message: '网站副标题不能超过 120 个字符', trigger: 'blur' }],
    stationQQ: [
      { max: 30, message: '站长 QQ 不能超过 30 个字符', trigger: 'blur' },
      { pattern: /^\d*$/, message: '站长 QQ 只能填写数字', trigger: 'blur' }
    ],
    icpNumber: [{ max: 100, message: '网站备案号不能超过 100 个字符', trigger: 'blur' }],
    domainLicenseNotice: [
      { max: 2000, message: '域名授权网站公告不能超过 2000 个字符', trigger: 'blur' }
    ]
  }

  onMounted(() => {
    loadConfig()
    loadRealnameConfig()
    loadRealnameRecords()
  })

  // 实名认证记录
  const recordList = ref<RealnameRecordItem[]>([])
  const recordTotal = ref(0)
  const recordLoading = ref(false)
  const recordQuery = reactive({
    page: 1,
    pageSize: 10,
    keyword: '',
    provider: '',
    status: '' as '' | 'passed' | 'failed'
  })

  const loadRealnameRecords = async () => {
    recordLoading.value = true
    try {
      const data = await fetchRealnameRecords({
        page: recordQuery.page,
        pageSize: recordQuery.pageSize,
        keyword: recordQuery.keyword || undefined,
        provider: recordQuery.provider || undefined,
        status: recordQuery.status || undefined
      })
      recordList.value = data.list || []
      recordTotal.value = data.total
    } catch {
      ElMessage.error('认证记录加载失败')
    } finally {
      recordLoading.value = false
    }
  }

  const handleRecordSearch = () => {
    recordQuery.page = 1
    loadRealnameRecords()
  }

  const realnameProviderName = (provider: string) => {
    if (provider === 'kuaitong') return '快瞳'
    if (provider === 'tencent') return '靓仔聚合认证'
    if (provider === 'xiaomu') return '小沐聚合实名'
    return '支付宝'
  }

  const realnameProviderDescription = (provider: RealnameProvider) => {
    if (provider === 'kuaitong') return '支持姓名与身份证二要素核验，或扫码拍照完成人脸认证'
    if (provider === 'tencent') return '复用扫码拍照流程，调用靓仔聚合认证完成人证合一核验'
    if (provider === 'xiaomu') return '聚合三要素、活体人脸与微信实名，认证产品可在下方切换'
    return '金融级实人认证，用户扫码刷脸完成'
  }

  const realnameProviderIcon = (provider: RealnameProvider) => {
    if (provider === 'kuaitong' || provider === 'tencent') return 'ri:id-card-line'
    if (provider === 'xiaomu') return 'ri:shield-user-line'
    return 'ri:alipay-line'
  }

  const openTencentRealnameDocs = () => {
    window.open('https://real.4775.cn/openapi-doc.html#start', '_blank', 'noopener,noreferrer')
  }

  const loadRealnameConfig = async () => {
    try {
      const data = await fetchRealnameConfig()
      Object.assign(realnameConfig, data)
      Object.assign(realnameForm, {
        enabled: data.enabled,
        provider: data.provider || 'alipay',
        appId: data.appId,
        privateKey: '',
        alipayPublicKey: data.alipayPublicKey,
        gateway: data.gateway || 'https://openapi.alipay.com/gateway.do',
        kuaitongAccessKey: data.kuaitongAccessKey,
        kuaitongSecret: '',
        kuaitongAuthType: data.kuaitongAuthType || 'face',
        tencentApiKey: data.tencentApiKey || '',
        tencentApiSecret: '',
        tencentBaseUrl: data.tencentBaseUrl || 'https://real.4775.cn/common/openapi',
        tencentUsePackage: data.tencentUsePackage,
        xiaomuAppKey: data.xiaomuAppKey || '',
        xiaomuAppSecret: '',
        xiaomuBaseUrl: data.xiaomuBaseUrl || 'https://smapi.x1m1.cn',
        xiaomuProductMode: data.xiaomuProductMode || 'three_element',
        requireAppIds: [...data.requireAppIds]
      })
    } catch {
      ElMessage.error('实名认证配置加载失败')
    }
  }

  const toggleRequireApp = (id: number) => {
    const idx = realnameForm.requireAppIds.indexOf(id)
    if (idx >= 0) {
      realnameForm.requireAppIds.splice(idx, 1)
    } else {
      realnameForm.requireAppIds.push(id)
    }
  }

  const handleSaveRealname = async () => {
    realnameSaving.value = true
    try {
      const data = await fetchUpdateRealnameConfig({
        enabled: realnameForm.enabled,
        provider: realnameForm.provider,
        appId: realnameForm.appId,
        privateKey: realnameForm.privateKey || undefined,
        alipayPublicKey: realnameForm.alipayPublicKey,
        gateway: realnameForm.gateway,
        kuaitongAccessKey: realnameForm.kuaitongAccessKey,
        kuaitongSecret: realnameForm.kuaitongSecret || undefined,
        kuaitongAuthType: realnameForm.kuaitongAuthType,
        tencentApiKey: realnameForm.tencentApiKey,
        tencentApiSecret: realnameForm.tencentApiSecret || undefined,
        tencentBaseUrl: realnameForm.tencentBaseUrl,
        tencentUsePackage: realnameForm.tencentUsePackage,
        xiaomuAppKey: realnameForm.xiaomuAppKey,
        xiaomuAppSecret: realnameForm.xiaomuAppSecret || undefined,
        xiaomuBaseUrl: realnameForm.xiaomuBaseUrl,
        xiaomuProductMode: realnameForm.xiaomuProductMode,
        requireAppIds: realnameForm.requireAppIds
      })
      Object.assign(realnameConfig, { ...realnameConfig, ...data })
      realnameForm.privateKey = ''
      realnameForm.kuaitongSecret = ''
      realnameForm.tencentApiSecret = ''
      realnameForm.xiaomuAppSecret = ''
      ElMessage.success('实名认证配置已更新')
    } finally {
      realnameSaving.value = false
    }
  }

  const loadConfig = async () => {
    loading.value = true
    try {
      const data = await fetchSystemConfig()
      applyForm(data)
      systemConfigStore.applyConfig(data)
    } finally {
      loading.value = false
    }
  }

  const applyForm = (data: SystemConfigData) => {
    Object.assign(form, {
      siteName: data.siteName || '',
      siteSubtitle: data.siteSubtitle || '',
      siteLogo: data.siteLogo || '',
      installedAt: data.installedAt || '',
      stationQQ: data.stationQQ || '',
      icpNumber: data.icpNumber || '',
      domainLicenseNotice: data.domainLicenseNotice || '',
      registrationEnabled: data.registrationEnabled ?? true,
      selfPurchaseEnabled: data.selfPurchaseEnabled ?? true,
      piracyDetectionEnabled: data.piracyDetectionEnabled ?? false
    })
    savedConfig.value = { ...form }
  }

  // 自动保存单个功能开关；失败时恢复服务端已保存状态。
  const handleFeatureSwitchChange = async (
    key: SystemFeatureSwitchKey,
    value: string | number | boolean
  ) => {
    const enabled = Boolean(value)
    const previousValue = savedConfig.value[key]
    featureSwitchSavingState[key] = true
    try {
      const data = await fetchUpdateSystemFeatureSwitch(key, enabled)
      form[key] = data.enabled
      savedConfig.value[key] = data.enabled
      systemConfigStore.applyConfig({ ...savedConfig.value })
      ElMessage.success('功能开关已自动保存')
    } catch (error) {
      form[key] = previousValue
      const message = error instanceof Error ? error.message : '功能开关保存失败'
      ElMessage.error(`${message}，已恢复原状态`)
    } finally {
      featureSwitchSavingState[key] = false
    }
  }

  const handleLogoUpload = (file: File) => {
    if (!ACCEPTED_LOGO_TYPES.includes(file.type)) {
      ElMessage.error('Logo 仅支持 PNG、JPG 或 WebP 格式')
      return false
    }

    if (file.size > MAX_LOGO_SIZE) {
      ElMessage.error('Logo 大小必须在 512KB 以内')
      return false
    }

    const reader = new FileReader()
    reader.onload = () => {
      form.siteLogo = String(reader.result || '')
    }
    reader.readAsDataURL(file)
    return false
  }

  const resetLogo = () => {
    form.siteLogo = savedConfig.value.siteLogo || ''
  }

  const removeLogo = () => {
    form.siteLogo = ''
  }

  // 重置为服务端已保存的配置
  const handleReset = () => {
    Object.assign(form, { ...savedConfig.value })
    formRef.value?.clearValidate()
    ElMessage.info('已恢复到上次保存的配置')
  }

  // 右上角保存按钮按当前页签分派：实名页签保存实名配置，其余保存系统配置，
  // 避免一个页签的必填校验阻塞另一个页签的保存。
  const handleSave = async () => {
    if (activeTab.value === 'realname') {
      await handleSaveRealname()
      return
    }
    if (!formRef.value) return
    await formRef.value.validate()

    saving.value = true
    try {
      const data = await fetchUpdateSystemConfig({ ...form })
      applyForm(data)
      systemConfigStore.applyConfig(data)
      ElMessage.success('系统配置已更新')
    } finally {
      saving.value = false
    }
  }
</script>

<style scoped lang="scss">
  .system-config-page {
    .config-card {
      min-height: 100%;
      border: 0;
    }

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;

      .header-actions {
        display: flex;
        gap: 10px;
      }

      h2 {
        margin: 0;
        font-size: 20px;
        font-weight: 600;
        color: var(--art-gray-900);
      }

      p {
        margin: 6px 0 0;
        font-size: 13px;
        color: var(--art-gray-600);
      }
    }

    .config-form {
      :deep(.el-input),
      :deep(.el-textarea) {
        width: 100%;
      }
    }

    .config-tabs {
      :deep(.el-tabs__header) {
        margin-bottom: 20px;
      }

      :deep(.el-tabs__item) {
        font-size: 14px;
      }
    }

    .section-card {
      padding: 22px;
      background: var(--art-main-bg-color);
      border: 1px solid var(--art-border-color);
      border-radius: 16px;
    }

    .notice-preview-panel {
      position: sticky;
      top: 20px;
      padding: 18px;
      background: var(--art-gray-100);
      border-radius: 14px;
    }

    .notice-preview-full {
      margin-top: 0;
      background: transparent;
      padding: 0;

      p {
        margin-top: 0;
      }
    }

    .section-title {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
      margin-bottom: 20px;

      strong,
      span {
        display: block;
      }

      strong {
        font-size: 16px;
        color: var(--art-gray-900);
      }

      span {
        margin-top: 6px;
        font-size: 13px;
        color: var(--art-gray-600);
      }
    }

    .realname-master-switch {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 16px 18px;
      margin-bottom: 24px;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 12px;
      transition:
        border-color 0.2s,
        background 0.2s;

      &.is-enabled {
        background: var(--el-color-success-light-9);
        border-color: var(--el-color-success-light-5);
      }

      .switch-content {
        display: flex;
        align-items: center;
        gap: 14px;
      }

      .switch-icon {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 40px;
        height: 40px;
        font-size: 20px;
        color: var(--el-color-primary);
        background: var(--default-box-color);
        border-radius: 10px;
        flex-shrink: 0;
      }

      strong,
      span {
        display: block;
      }

      strong {
        font-size: 15px;
        color: var(--art-gray-900);
      }

      span {
        margin-top: 4px;
        font-size: 13px;
        color: var(--art-gray-600);
      }
    }

    .realname-body {
      margin-bottom: 8px;
    }

    .config-block {
      padding: 18px;
      margin-bottom: 18px;
      background: var(--art-gray-100);
      border: 1px solid var(--art-border-color);
      border-radius: 12px;

      &:last-child {
        margin-bottom: 0;
      }
    }

    .block-header {
      display: flex;
      align-items: center;
      gap: 12px;
      margin-bottom: 18px;

      .block-num {
        display: flex;
        align-items: center;
        justify-content: center;
        width: 26px;
        height: 26px;
        font-size: 13px;
        font-weight: 600;
        color: #fff;
        background: var(--el-color-primary);
        border-radius: 8px;
        flex-shrink: 0;
      }

      .help-icon {
        font-size: 20px;
        color: var(--el-color-primary);
      }

      .block-count {
        flex-shrink: 0;
        margin-left: auto;
      }

      strong,
      span {
        display: block;
      }

      strong {
        font-size: 14px;
        color: var(--art-gray-900);
      }

      span {
        margin-top: 3px;
        font-size: 12px;
        color: var(--art-gray-600);
      }
    }

    .input-icon {
      font-size: 15px;
      color: var(--art-gray-500);
    }

    .label-tag {
      margin-left: 8px;
      vertical-align: 2px;
    }

    .field-hint {
      display: flex;
      align-items: center;
      gap: 4px;
      margin-top: 6px;
      font-size: 12px;
      color: var(--art-gray-600);

      &.is-error {
        color: var(--el-color-danger);
      }
    }

    .provider-select {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .plugin-off-alert {
      border-radius: 10px;
    }

    .block-link-btn {
      flex-shrink: 0;
      margin-left: auto;
    }

    .provider-item {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 14px 16px;
      background: var(--default-box-color);
      border: 1px solid var(--art-border-color);
      border-radius: 10px;
      cursor: pointer;
      transition:
        border-color 0.15s,
        background 0.15s;

      &:not(.is-static):hover {
        border-color: var(--el-color-primary-light-5);
      }

      &.is-static {
        cursor: default;
      }

      &.is-active {
        background: var(--el-color-primary-light-9);
        border-color: var(--el-color-primary);
      }

      .provider-icon {
        flex-shrink: 0;
        font-size: 26px;

        &.alipay {
          color: #1677ff;
        }

        &.kuaitong {
          color: #00b42a;
        }

        &.tencent {
          color: #0052d9;
        }
      }

      .provider-info {
        flex: 1;
        min-width: 0;

        strong,
        span {
          display: block;
        }

        strong {
          font-size: 14px;
          color: var(--art-gray-900);
        }

        span {
          margin-top: 3px;
          font-size: 12px;
          color: var(--art-gray-600);
        }
      }

      .provider-check {
        flex-shrink: 0;
        font-size: 20px;
        color: var(--el-color-primary);
      }
    }

    .app-select-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .app-select-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      background: var(--default-box-color);
      border: 1px solid var(--art-border-color);
      border-radius: 10px;
      cursor: pointer;
      transition:
        border-color 0.15s,
        background 0.15s;

      &:hover {
        border-color: var(--el-color-primary-light-5);
      }

      &.is-checked {
        background: var(--el-color-warning-light-9);
        border-color: var(--el-color-warning-light-5);
      }

      .app-info {
        flex: 1;
        min-width: 0;

        strong,
        span {
          display: block;
        }

        strong {
          font-size: 13px;
          color: var(--art-gray-900);
        }

        span {
          margin-top: 2px;
          font-size: 12px;
          color: var(--art-gray-500);
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
      }
    }

    .app-empty-state {
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 32px 0;
      color: var(--art-gray-500);

      svg {
        font-size: 32px;
        margin-bottom: 8px;
      }

      p {
        margin: 0 0 4px;
        font-size: 14px;
      }
    }

    .help-block {
      background: var(--el-color-primary-light-9);
      border-color: var(--el-color-primary-light-7);
    }

    .help-list {
      padding-left: 18px;
      margin: 0;

      li {
        margin-bottom: 8px;
        font-size: 13px;
        line-height: 1.6;
        color: var(--art-gray-700);

        &:last-child {
          margin-bottom: 0;
        }

        code {
          padding: 2px 6px;
          font-size: 12px;
          background: var(--default-box-color);
          border-radius: 4px;
        }
      }
    }

    .switch-list {
      overflow: hidden;
      border: 1px solid var(--art-border-color);
      border-radius: 12px;
    }

    .switch-item {
      display: flex;
      align-items: center;
      justify-content: flex-start;
      gap: 16px;
      padding: 18px;

      > div {
        flex: 0 1 320px;
      }

      & + .switch-item {
        border-top: 1px solid var(--art-border-color);
      }

      strong,
      span {
        display: block;
      }

      strong {
        font-size: 14px;
        color: var(--art-gray-900);
      }

      span {
        margin-top: 5px;
        font-size: 13px;
        color: var(--art-gray-600);
      }

      :deep(.el-switch) {
        flex-shrink: 0;
      }
    }

    .logo-uploader-wrap {
      display: flex;
      gap: 18px;
      align-items: center;
      flex-wrap: wrap;
    }

    .logo-uploader {
      position: relative;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 112px;
      height: 112px;
      overflow: hidden;
      cursor: pointer;
      background: var(--art-gray-100);
      border: 1px dashed var(--art-gray-400);
      border-radius: 18px;

      img {
        max-width: 72px;
        max-height: 72px;
        object-fit: contain;
      }

      .upload-mask {
        position: absolute;
        inset: 0;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        gap: 6px;
        font-size: 13px;
        color: #fff;
        background: rgba(0, 0, 0, 0.55);
        opacity: 0;
        transition: opacity 0.2s;
      }

      &:hover .upload-mask {
        opacity: 1;
      }
    }

    .logo-actions {
      display: flex;
      flex-direction: column;
      gap: 12px;
    }

    .action-buttons {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
    }

    .preview-panel {
      position: sticky;
      top: 20px;
      padding: 22px;
      background: var(--art-gray-100);
      border-radius: 18px;
    }

    .preview-title {
      margin-bottom: 14px;
      font-size: 15px;
      font-weight: 600;
      color: var(--art-gray-800);
    }

    .preview-card {
      display: flex;
      gap: 14px;
      align-items: center;
      padding: 18px;
      margin-bottom: 18px;
      background: var(--default-box-color);
      border-radius: 16px;
      box-shadow: var(--el-box-shadow-light);

      img {
        width: 44px;
        height: 44px;
        object-fit: contain;
        border-radius: 10px;
      }

      strong,
      span {
        display: block;
      }

      strong {
        margin-bottom: 4px;
        font-size: 17px;
        color: var(--art-gray-900);
      }

      span {
        font-size: 13px;
        color: var(--art-gray-600);
      }
    }

    .notice-preview {
      padding: 16px;
      margin-top: 18px;
      background: var(--default-box-color);
      border-radius: 12px;

      strong {
        font-size: 14px;
        color: var(--art-gray-900);
      }

      p {
        margin: 8px 0 0;
        font-size: 13px;
        line-height: 1.7;
        color: var(--art-gray-600);
        white-space: pre-wrap;
        overflow-wrap: anywhere;
      }
    }

    @media (max-width: 991px) {
      .preview-panel {
        position: static;
      }
    }
  }

  .realname-records {
    margin-top: 20px;
  }

  .record-filters {
    display: flex;
    gap: 12px;
    flex-wrap: wrap;
    margin-bottom: 14px;
  }

  .record-owner {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .record-email {
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }

  .record-fail-reason {
    line-height: 1.6;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .record-pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 14px;
  }
</style>
