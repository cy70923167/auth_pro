<template>
  <div class="license-home">
    <header class="site-header">
      <div class="nav-shell">
        <button class="brand-button" type="button" @click="scrollToSection('home')">
          <span class="brand-logo-wrap">
            <img :src="resolvedLogo" class="brand-logo" alt="网站 Logo" />
          </span>
          <span class="brand-copy">
            <strong>{{ siteName }}</strong>
            <small>License Service</small>
          </span>
        </button>

        <nav class="main-nav" aria-label="首页导航">
          <button type="button" @click="scrollToSection('home')">首页</button>
          <button type="button" @click="scrollToSection('query-card')">快速查询</button>
          <button type="button" @click="scrollToSection('features')">平台能力</button>
          <button type="button" @click="scrollToSection('workflow')">使用流程</button>
        </nav>

        <div class="header-actions">
          <el-button class="header-query" plain @click="scrollToSection('query-card')">
            <IconifyIcon icon="ri:search-eye-line" />
            快速查询
          </el-button>
          <el-button class="header-login" text @click="openAuthDialog('login')">登录</el-button>
          <el-button v-if="registrationEnabled" type="primary" @click="openAuthDialog('register')">
            注册
          </el-button>
        </div>
      </div>
    </header>

    <main>
      <section id="home" class="hero-section">
        <div class="hero-orb hero-orb-one" />
        <div class="hero-orb hero-orb-two" />
        <div class="page-shell hero-grid">
          <div class="hero-content">
            <div class="hero-badge">
              <span class="badge-dot" />
              专业、稳定的软件授权服务
            </div>
            <h1>
              让每一份软件授权
              <span>清晰、安全、可管理</span>
            </h1>
            <p class="hero-description">
              {{
                siteSubtitle
              }}。从授权开通、状态查询到在线续费，为软件服务提供完整、可靠的授权管理体验。
            </p>
            <div class="hero-actions">
              <el-button type="primary" size="large" @click="openAuthDialog('login')">
                <IconifyIcon icon="ri:user-line" />
                进入用户中心
                <IconifyIcon icon="ri:arrow-right-line" />
              </el-button>
              <el-button size="large" plain @click="scrollToSection('query-card')">
                <IconifyIcon icon="ri:search-line" />
                快速查询服务
              </el-button>
            </div>
            <div class="hero-trust">
              <div>
                <IconifyIcon icon="ri:shield-check-fill" />
                <span>授权状态实时同步</span>
              </div>
              <div>
                <IconifyIcon icon="ri:lock-2-fill" />
                <span>账户信息安全保护</span>
              </div>
              <div>
                <IconifyIcon icon="ri:time-fill" />
                <span>7×24 小时在线服务</span>
              </div>
            </div>
          </div>

          <div class="hero-visual" aria-label="授权服务概览">
            <div class="visual-glow" />
            <div class="dashboard-card">
              <div class="dashboard-head">
                <div class="window-dots"><i /><i /><i /></div>
                <span>授权服务中心</span>
                <span class="online-pill"><i /> 服务正常</span>
              </div>
              <div class="dashboard-body">
                <div class="summary-row">
                  <div class="summary-icon">
                    <IconifyIcon icon="ri:shield-check-line" />
                  </div>
                  <div>
                    <small>授权状态</small>
                    <strong>授权有效</strong>
                  </div>
                  <span class="success-tag">运行中</span>
                </div>
                <div class="license-preview">
                  <div class="preview-title">
                    <span class="app-mark"><IconifyIcon icon="ri:apps-2-line" /></span>
                    <div>
                      <strong>应用授权服务</strong>
                      <small>专业版授权套餐</small>
                    </div>
                  </div>
                  <div class="preview-grid">
                    <div><small>授权类型</small><span>单域名</span></div>
                    <div><small>开通时间</small><span>实时生效</span></div>
                    <div><small>授权验证</small><span>安全可靠</span></div>
                    <div><small>状态同步</small><span>自动完成</span></div>
                  </div>
                </div>
                <div class="activity-row">
                  <span><i /> 最近一次状态同步成功</span>
                  <small>刚刚</small>
                </div>
              </div>
            </div>
            <div class="floating-card floating-card-top">
              <IconifyIcon icon="ri:flashlight-fill" />
              <div><strong>秒级验证</strong><small>稳定响应</small></div>
            </div>
            <div class="floating-card floating-card-bottom">
              <IconifyIcon icon="ri:database-2-fill" />
              <div><strong>数据可追溯</strong><small>状态清晰</small></div>
            </div>
          </div>
        </div>
      </section>

      <section id="query-card" class="query-section section-block">
        <div class="page-shell">
          <div class="section-heading centered-heading">
            <span class="section-kicker">QUICK LOOKUP</span>
            <h2>快速查询服务</h2>
            <p>查询授权状态或验证代理商身份，一键切换查询类型。</p>
            <div class="flip-toggle-buttons">
              <button
                type="button"
                :class="['flip-toggle-btn', { active: !isFlipped }]"
                @click="isFlipped = false"
              >
                <IconifyIcon icon="ri:shield-check-line" />
                授权查询
              </button>
              <button
                type="button"
                :class="['flip-toggle-btn', { active: isFlipped }]"
                @click="isFlipped = true"
              >
                <IconifyIcon icon="ri:user-star-line" />
                代理商查询
              </button>
            </div>
          </div>

          <div class="flip-card-container" :class="{ 'is-flipped': isFlipped }">
            <div class="flip-card">
              <!-- 正面：授权查询 -->
              <div class="flip-card-face flip-card-front">
                <div class="query-panel">
                  <div class="query-input-row">
                    <el-input
                      v-model="licenseQueryAccount"
                      size="large"
                      clearable
                      placeholder="请输入用户账号或注册邮箱"
                      @keyup.enter="handleLicenseQuery"
                    >
                      <template #prefix>
                        <IconifyIcon icon="ri:user-search-line" />
                      </template>
                    </el-input>
                    <el-button
                      type="primary"
                      size="large"
                      :loading="licenseQueryLoading"
                      @click="handleLicenseQuery"
                    >
                      <IconifyIcon icon="ri:search-line" />
                      立即查询
                    </el-button>
                  </div>
                  <div class="query-security-tip">
                    <IconifyIcon icon="ri:information-line" />
                    查询结果仅展示授权概况，不展示授权密钥、绑定目标或账户隐私信息。
                  </div>

                  <div v-if="licenseQuerySearched" class="query-results">
                    <div v-if="licenseQueryList.length" class="result-summary">
                      <div>
                        <span class="result-icon"
                          ><IconifyIcon icon="ri:checkbox-circle-fill"
                        /></span>
                        <div>
                          <strong>查询完成</strong>
                          <small>共找到 {{ licenseQueryTotal }} 条授权记录</small>
                        </div>
                      </div>
                      <span v-if="licenseQueryTotal > licenseQueryList.length" class="result-limit">
                        当前展示前 {{ licenseQueryList.length }} 条
                      </span>
                    </div>

                    <div v-if="licenseQueryList.length" class="license-result-grid">
                      <article
                        v-for="(item, index) in licenseQueryList"
                        :key="`${item.appName}-${item.openedAt}-${index}`"
                        class="license-result-card"
                      >
                        <div class="result-card-head">
                          <div class="result-app-icon"
                            ><IconifyIcon icon="ri:shield-keyhole-line"
                          /></div>
                          <div class="result-app-name">
                            <strong>{{ item.appName || '未命名应用' }}</strong>
                            <span>{{ item.planName }}</span>
                          </div>
                          <span class="status-badge" :class="`is-${item.status}`">
                            {{ item.statusName || item.status }}
                          </span>
                        </div>
                        <div class="result-meta-grid">
                          <div>
                            <small>授权类型</small>
                            <strong>{{ item.licenseTypeName }}</strong>
                          </div>
                          <div>
                            <small>授权套餐</small>
                            <strong>{{ item.planName }}</strong>
                          </div>
                          <div>
                            <small>开通时间</small>
                            <strong>{{ item.openedAt }}</strong>
                          </div>
                          <div>
                            <small>到期时间</small>
                            <strong>{{ item.permanent ? '永久有效' : item.expiredAt }}</strong>
                          </div>
                        </div>
                      </article>
                    </div>

                    <div v-else class="query-empty">
                      <span><IconifyIcon icon="ri:inbox-2-line" /></span>
                      <strong>暂未查询到授权记录</strong>
                      <p>请确认账号或邮箱输入正确，也可以登录用户中心查看完整信息。</p>
                      <el-button plain @click="openAuthDialog('login')">前往登录</el-button>
                    </div>
                  </div>
                </div>
              </div>

              <!-- 反面：代理商查询 -->
              <div class="flip-card-face flip-card-back">
                <div class="query-panel">
                  <div class="query-input-row">
                    <el-input
                      v-model="agentQueryAccount"
                      size="large"
                      clearable
                      placeholder="请输入代理商账号"
                      @keyup.enter="handleAgentQuery"
                    >
                      <template #prefix>
                        <IconifyIcon icon="ri:user-star-line" />
                      </template>
                    </el-input>
                    <el-button
                      type="primary"
                      size="large"
                      :loading="agentQueryLoading"
                      @click="handleAgentQuery"
                    >
                      <IconifyIcon icon="ri:search-line" />
                      立即查询
                    </el-button>
                  </div>
                  <div class="query-security-tip">
                    <IconifyIcon icon="ri:information-line" />
                    查询结果仅展示代理商账号与代理等级，不展示余额、联系方式等隐私信息。
                  </div>

                  <div v-if="agentQuerySearched" class="query-results">
                    <div v-if="agentQueryResult" class="license-result-grid">
                      <article class="license-result-card">
                        <div class="result-card-head">
                          <div class="result-app-icon"
                            ><IconifyIcon icon="ri:user-star-line"
                          /></div>
                          <div class="result-app-name">
                            <strong>{{ agentQueryResult.agentName }}</strong>
                            <span>代理商账号</span>
                          </div>
                          <span class="status-badge is-active">代理商</span>
                        </div>
                        <div class="result-meta-grid">
                          <div>
                            <small>代理商账号</small>
                            <strong>{{ agentQueryResult.account }}</strong>
                          </div>
                          <div>
                            <small>代理商级别</small>
                            <strong>{{ agentQueryResult.levelName }}</strong>
                          </div>
                        </div>
                      </article>
                    </div>

                    <div v-else class="query-empty">
                      <span><IconifyIcon icon="ri:inbox-2-line" /></span>
                      <strong>未查询到当前账号</strong>
                      <p>请确认代理商账号输入正确，该账号可能不是代理商或已被禁用。</p>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section id="features" class="features-section section-block">
        <div class="page-shell">
          <div class="section-heading">
            <span class="section-kicker">PLATFORM CAPABILITIES</span>
            <h2>围绕软件授权打造的一站式服务</h2>
            <p>覆盖授权生命周期中的关键环节，让授权购买、使用和管理更简单。</p>
          </div>
          <div class="feature-grid">
            <article v-for="feature in featureItems" :key="feature.title" class="feature-card">
              <span class="feature-card-icon"><IconifyIcon :icon="feature.icon" /></span>
              <h3>{{ feature.title }}</h3>
              <p>{{ feature.description }}</p>
              <div class="feature-link"
                >{{ feature.tag }} <IconifyIcon icon="ri:arrow-right-up-line"
              /></div>
            </article>
          </div>
        </div>
      </section>

      <section id="workflow" class="workflow-section section-block">
        <div class="page-shell workflow-shell">
          <div class="section-heading workflow-heading">
            <span class="section-kicker">HOW IT WORKS</span>
            <h2>三步开启授权服务</h2>
            <p>清晰直接的使用流程，无需复杂配置即可完成授权管理。</p>
            <el-button
              type="primary"
              size="large"
              @click="openAuthDialog(registrationEnabled ? 'register' : 'login')"
            >
              {{ registrationEnabled ? '免费注册账号' : '登录用户中心' }}
              <IconifyIcon icon="ri:arrow-right-line" />
            </el-button>
          </div>
          <div class="workflow-list">
            <article v-for="(step, index) in workflowItems" :key="step.title" class="workflow-item">
              <span class="step-number">0{{ index + 1 }}</span>
              <span class="step-icon"><IconifyIcon :icon="step.icon" /></span>
              <div>
                <h3>{{ step.title }}</h3>
                <p>{{ step.description }}</p>
              </div>
            </article>
          </div>
        </div>
      </section>

      <section class="cta-section">
        <div class="page-shell cta-card">
          <div>
            <span class="section-kicker">GET STARTED</span>
            <h2>开始管理您的软件授权</h2>
            <p>登录用户中心，集中查看授权状态、有效时间和账户信息。</p>
          </div>
          <div class="cta-actions">
            <el-button size="large" plain @click="scrollToSection('query-card')"
              >查询授权</el-button
            >
            <el-button type="primary" size="large" @click="openAuthDialog('login')"
              >立即登录</el-button
            >
          </div>
        </div>
      </section>
    </main>

    <footer class="site-footer">
      <div class="page-shell footer-shell">
        <div class="footer-brand">
          <img :src="resolvedLogo" alt="网站 Logo" />
          <div
            ><strong>{{ siteName }}</strong
            ><span>{{ siteSubtitle }}</span></div
          >
        </div>
        <div class="footer-meta">
          <span v-if="stationQQ">服务联系 QQ：{{ stationQQ }}</span>
          <a
            v-if="icpNumber"
            href="https://beian.miit.gov.cn/"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ icpNumber }}
          </a>
          <span>© {{ currentYear }} {{ siteName }}</span>
        </div>
      </div>
    </footer>

    <el-dialog
      v-model="authDialogVisible"
      :width="
        mode === 'register' ? 'min(640px, calc(100vw - 28px))' : 'min(480px, calc(100vw - 28px))'
      "
      align-center
      append-to-body
      class="home-auth-dialog"
      :show-close="true"
      :close-on-click-modal="false"
    >
      <template #header>
        <div class="auth-brand">
          <img :src="resolvedLogo" alt="网站 Logo" />
          <div
            ><strong>{{ siteName }}</strong
            ><span>用户服务中心</span></div
          >
        </div>
      </template>

      <transition name="card-fade" mode="out-in">
        <div v-if="mode === 'login'" key="login" class="auth-form-wrapper">
          <h2>欢迎回来</h2>
          <p class="auth-subtitle">登录后查看并管理您的全部授权</p>
          <el-form
            ref="loginFormRef"
            :model="loginForm"
            :rules="loginRules"
            class="auth-form"
            @keyup.enter="handleLogin"
          >
            <el-form-item prop="username">
              <el-input
                v-model="loginForm.username"
                placeholder="手机号 / 邮箱 / 用户ID"
                size="large"
              >
                <template #prefix><IconifyIcon icon="ri:user-line" /></template>
              </el-input>
            </el-form-item>
            <el-form-item prop="password">
              <el-input
                v-model="loginForm.password"
                type="password"
                placeholder="登录密码"
                size="large"
                show-password
              >
                <template #prefix><IconifyIcon icon="ri:lock-line" /></template>
              </el-input>
            </el-form-item>
            <div class="form-options">
              <el-checkbox v-model="loginForm.remember">记住我</el-checkbox>
              <el-link type="primary" :underline="false" @click="mode = 'forgot'"
                >忘记密码？</el-link
              >
            </div>
            <el-button
              class="auth-submit"
              type="primary"
              size="large"
              :loading="loading"
              @click="handleLogin"
              >登录用户中心</el-button
            >
          </el-form>
          <div class="auth-switch">
            <template v-if="registrationEnabled">
              <span>还没有账号？</span>
              <el-link type="primary" :underline="false" @click="mode = 'register'"
                >立即注册</el-link
              >
            </template>
            <span v-else>普通用户注册已关闭，请联系管理员</span>
          </div>
        </div>

        <div v-else-if="mode === 'register'" key="register" class="auth-form-wrapper">
          <h2>注册新用户</h2>
          <p class="auth-subtitle">创建账号，开始使用授权服务</p>
          <el-form
            ref="registerFormRef"
            :model="registerForm"
            :rules="registerRules"
            class="auth-form register-form"
            @keyup.enter="handleRegister"
          >
            <div class="form-grid-two">
              <el-form-item prop="email">
                <el-input v-model="registerForm.email" placeholder="邮箱地址" size="large">
                  <template #prefix><IconifyIcon icon="ri:mail-line" /></template>
                </el-input>
              </el-form-item>
              <el-form-item prop="phone">
                <el-input v-model="registerForm.phone" placeholder="手机号（选填）" size="large">
                  <template #prefix><IconifyIcon icon="ri:smartphone-line" /></template>
                </el-input>
              </el-form-item>
            </div>
            <el-form-item prop="nickname">
              <el-input v-model="registerForm.nickname" placeholder="用户账号" size="large">
                <template #prefix><IconifyIcon icon="ri:user-line" /></template>
              </el-input>
            </el-form-item>
            <div class="form-grid-two">
              <el-form-item prop="password">
                <el-input
                  v-model="registerForm.password"
                  type="password"
                  placeholder="设置密码"
                  size="large"
                  show-password
                >
                  <template #prefix><IconifyIcon icon="ri:lock-line" /></template>
                </el-input>
              </el-form-item>
              <el-form-item prop="confirmPassword">
                <el-input
                  v-model="registerForm.confirmPassword"
                  type="password"
                  placeholder="确认密码"
                  size="large"
                  show-password
                >
                  <template #prefix><IconifyIcon icon="ri:lock-line" /></template>
                </el-input>
              </el-form-item>
            </div>
            <el-form-item prop="emailCode">
              <div class="email-code-row">
                <el-input
                  v-model="registerForm.emailCode"
                  placeholder="6 位邮箱验证码"
                  size="large"
                  maxlength="6"
                  inputmode="numeric"
                >
                  <template #prefix><IconifyIcon icon="ri:shield-keyhole-line" /></template>
                </el-input>
                <el-button
                  size="large"
                  :loading="emailCodeSending"
                  :disabled="emailCodeCountdown > 0"
                  @click="handleRequestEmailCode"
                >
                  {{ emailCodeCountdown > 0 ? `${emailCodeCountdown} 秒` : '获取验证码' }}
                </el-button>
              </div>
            </el-form-item>
            <el-button
              class="auth-submit"
              type="primary"
              size="large"
              :loading="loading"
              @click="handleRegister"
              >注册并进入用户中心</el-button
            >
          </el-form>
          <div class="auth-switch">
            <span>已有账号？</span>
            <el-link type="primary" :underline="false" @click="mode = 'login'">返回登录</el-link>
          </div>
        </div>

        <div v-else key="forgot" class="auth-form-wrapper">
          <h2>找回密码</h2>
          <p class="auth-subtitle">输入注册邮箱，我们将发送密码重置链接</p>
          <el-form
            ref="forgotFormRef"
            :model="forgotForm"
            :rules="forgotRules"
            class="auth-form"
            @keyup.enter="handleForgot"
          >
            <el-form-item prop="email">
              <el-input v-model="forgotForm.email" placeholder="注册邮箱" size="large">
                <template #prefix><IconifyIcon icon="ri:mail-line" /></template>
              </el-input>
            </el-form-item>
            <el-button
              class="auth-submit"
              type="primary"
              size="large"
              :loading="loading"
              @click="handleForgot"
              >发送重置链接</el-button
            >
          </el-form>
          <div class="auth-switch">
            <span>想起密码了？</span>
            <el-link type="primary" :underline="false" @click="mode = 'login'">返回登录</el-link>
          </div>
        </div>
      </transition>
    </el-dialog>

    <el-dialog
      v-model="emailCaptchaVisible"
      title="安全验证"
      width="min(420px, calc(100vw - 32px))"
      align-center
      append-to-body
      :close-on-click-modal="false"
      @closed="handleEmailCaptchaClosed"
    >
      <div class="email-captcha-content">
        <p
          >请拖动滑块完成验证，验证通过后将向
          {{ registerForm.email.trim() || '注册邮箱' }} 发送验证码。</p
        >
        <div class="email-captcha-slider">
          <ArtDragVerify
            ref="emailCaptchaRef"
            v-model:value="emailCaptchaPassing"
            text="按住滑块拖动到底"
            success-text="验证通过，正在发送..."
            text-color="var(--el-text-color-secondary)"
            progress-bar-bg="var(--el-color-primary)"
            background="var(--el-fill-color-lighter)"
            handler-bg="var(--el-bg-color)"
            :height="46"
            @pass-callback="handleEmailCaptchaPassed"
          />
        </div>
      </div>
    </el-dialog>

    <el-dialog
      v-model="announcementVisible"
      title="网站公告"
      width="min(480px, calc(100vw - 32px))"
      align-center
      append-to-body
      @closed="handleAnnouncementClosed"
    >
      <div class="announcement-content">{{ domainLicenseNotice }}</div>
      <template #footer>
        <div class="announcement-footer">
          <el-checkbox v-model="hideAnnouncementToday">今日不再显示</el-checkbox>
          <el-button type="primary" @click="announcementVisible = false">我知道了</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
  import { ref, reactive, watch, onMounted, onBeforeUnmount, nextTick, computed } from 'vue'
  import { useRouter, useRoute } from 'vue-router'
  import { ElMessage, type FormRules } from 'element-plus'
  import { Icon as IconifyIcon } from '@iconify/vue'
  import axios from 'axios'
  import { useSystemConfigStore } from '@/store/modules/system-config'

  interface PublicLicenseItem {
    appName: string
    planName: string
    licenseType: string
    licenseTypeName: string
    status: string
    statusName: string
    openedAt: string
    expiredAt: string | null
    permanent: boolean
  }

  interface PublicAgentItem {
    account: string
    agentName: string
    levelName: string
  }

  const featureItems = [
    {
      icon: 'ri:shield-check-line',
      title: '多类型授权',
      description: '支持单域名、泛域名、IP 与密钥授权，覆盖不同的软件部署场景。',
      tag: '灵活适配'
    },
    {
      icon: 'ri:search-eye-line',
      title: '授权状态查询',
      description: '通过账号或邮箱快速了解授权应用、套餐和授权有效时间。',
      tag: '信息清晰'
    },
    {
      icon: 'ri:shopping-bag-3-line',
      title: '在线购买授权',
      description: '用户可在服务中心选择应用与套餐，自助完成授权购买和开通。',
      tag: '便捷开通'
    },
    {
      icon: 'ri:refresh-line',
      title: '实时状态同步',
      description: '授权状态、续费结果和有效期实时更新，减少人工确认成本。',
      tag: '实时可靠'
    },
    {
      icon: 'ri:lock-password-line',
      title: '安全账户体系',
      description: '邮箱验证、登录保护与密码重置流程，共同保障账户访问安全。',
      tag: '安全防护'
    },
    {
      icon: 'ri:customer-service-2-line',
      title: '持续服务支持',
      description: '从授权开通到日常使用，为用户提供连贯、稳定的服务体验。',
      tag: '服务保障'
    }
  ]

  const workflowItems = [
    {
      icon: 'ri:user-add-line',
      title: '创建用户账号',
      description: '使用邮箱完成验证注册，建立安全的用户服务账户。'
    },
    {
      icon: 'ri:shopping-cart-2-line',
      title: '选择授权方案',
      description: '根据应用和使用场景选择适合的授权类型与服务套餐。'
    },
    {
      icon: 'ri:dashboard-line',
      title: '统一管理授权',
      description: '在用户中心集中查看授权状态、有效时间并完成后续管理。'
    }
  ]

  const router = useRouter()
  const route = useRoute()
  const systemConfigStore = useSystemConfigStore()
  const {
    siteName,
    siteSubtitle,
    resolvedLogo,
    domainLicenseNotice,
    stationQQ,
    icpNumber,
    registrationEnabled
  } = storeToRefs(systemConfigStore)

  const currentYear = computed(() => new Date().getFullYear())
  const authDialogVisible = ref(false)
  const loading = ref(false)
  const mode = ref<'login' | 'register' | 'forgot'>('login')
  const loginFormRef = ref()
  const registerFormRef = ref()
  const forgotFormRef = ref()
  const announcementVisible = ref(false)
  const hideAnnouncementToday = ref(false)
  const ANNOUNCEMENT_HIDE_DATE_KEY = 'user-panel-announcement-hidden-date'

  const licenseQueryAccount = ref('')
  const licenseQueryLoading = ref(false)
  const licenseQuerySearched = ref(false)
  const licenseQueryList = ref<PublicLicenseItem[]>([])
  const licenseQueryTotal = ref(0)

  const agentQueryAccount = ref('')
  const agentQueryLoading = ref(false)
  const agentQuerySearched = ref(false)
  const agentQueryResult = ref<PublicAgentItem | null>(null)

  const isFlipped = ref(false)

  function scrollToSection(id: string) {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  function openAuthDialog(targetMode: 'login' | 'register') {
    if (targetMode === 'register' && !registrationEnabled.value) {
      ElMessage.warning('普通用户注册已关闭，请联系管理员')
      targetMode = 'login'
    }
    mode.value = targetMode
    authDialogVisible.value = true
    nextTick(() => {
      loginFormRef.value?.clearValidate()
      registerFormRef.value?.clearValidate()
      forgotFormRef.value?.clearValidate()
    })
  }

  async function handleLicenseQuery() {
    const account = licenseQueryAccount.value.trim()
    if (!account) {
      ElMessage.warning('请输入用户账号或注册邮箱')
      return
    }

    licenseQueryLoading.value = true
    try {
      const { data } = await axios.get('/api/user-panel/license-query', {
        params: { account, page: 1, pageSize: 50 }
      })
      if (data.code === 200) {
        licenseQueryList.value = Array.isArray(data.data?.list) ? data.data.list : []
        licenseQueryTotal.value = Number(data.data?.total || 0)
        licenseQuerySearched.value = true
      } else {
        licenseQueryList.value = []
        licenseQueryTotal.value = 0
        licenseQuerySearched.value = false
        ElMessage.error(data.msg || '授权查询失败')
      }
    } catch {
      ElMessage.error('网络错误，请稍后重试')
    } finally {
      licenseQueryLoading.value = false
    }
  }

  async function handleAgentQuery() {
    const account = agentQueryAccount.value.trim()
    if (!account) {
      ElMessage.warning('请输入代理商账号')
      return
    }

    agentQueryLoading.value = true
    try {
      const { data } = await axios.get('/api/user-panel/agent-query', {
        params: { account }
      })
      if (data.code === 200) {
        agentQueryResult.value = data.data?.found ? (data.data as PublicAgentItem) : null
        agentQuerySearched.value = true
      } else {
        agentQueryResult.value = null
        agentQuerySearched.value = false
        ElMessage.error(data.msg || '代理商查询失败')
      }
    } catch {
      ElMessage.error('网络错误，请稍后重试')
    } finally {
      agentQueryLoading.value = false
    }
  }

  onMounted(() => {
    if (route.query.impersonate !== '1') return
    const raw = sessionStorage.getItem('impersonate_user_panel')
    if (!raw) return
    try {
      const info = JSON.parse(raw)
      localStorage.setItem('user_panel_token', info.accessToken)
      localStorage.setItem('user_panel_info', raw)
      sessionStorage.removeItem('impersonate_user_panel')
      ElMessage.success('已登录该用户账号')
      router.replace('/user/dashboard')
    } catch {
      sessionStorage.removeItem('impersonate_user_panel')
    }
  })

  function getLocalDateKey() {
    const today = new Date()
    const year = today.getFullYear()
    const month = String(today.getMonth() + 1).padStart(2, '0')
    const day = String(today.getDate()).padStart(2, '0')
    return `${year}-${month}-${day}`
  }

  function isAnnouncementHiddenToday() {
    try {
      return localStorage.getItem(ANNOUNCEMENT_HIDE_DATE_KEY) === getLocalDateKey()
    } catch {
      return false
    }
  }

  function updateAnnouncementVisibility(notice: string) {
    hideAnnouncementToday.value = false
    announcementVisible.value = Boolean(notice.trim()) && !isAnnouncementHiddenToday()
  }

  function handleAnnouncementClosed() {
    if (!hideAnnouncementToday.value) return
    try {
      localStorage.setItem(ANNOUNCEMENT_HIDE_DATE_KEY, getLocalDateKey())
    } catch {
      return
    }
  }

  watch(domainLicenseNotice, updateAnnouncementVisibility, { immediate: true })
  watch(
    registrationEnabled,
    (enabled) => {
      if (!enabled && mode.value === 'register') mode.value = 'login'
    },
    { immediate: true }
  )

  const loginForm = reactive({ username: '', password: '', remember: false })
  const loginRules = {
    username: [{ required: true, message: '请输入手机号、邮箱或用户ID', trigger: 'blur' }],
    password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
  }

  const registerForm = reactive({
    email: '',
    emailCode: '',
    phone: '',
    nickname: '',
    password: '',
    confirmPassword: ''
  })
  const emailCaptchaVisible = ref(false)
  const emailCaptchaPassing = ref(false)
  const emailCaptchaRef = ref()
  const emailCodeSending = ref(false)
  const emailCodeCountdown = ref(0)
  const emailCodeTarget = ref('')
  let emailCodeTimer: ReturnType<typeof setInterval> | undefined

  function stopEmailCodeCountdown() {
    if (emailCodeTimer) {
      clearInterval(emailCodeTimer)
      emailCodeTimer = undefined
    }
  }

  function startEmailCodeCountdown() {
    stopEmailCodeCountdown()
    emailCodeCountdown.value = 60
    emailCodeTimer = setInterval(() => {
      emailCodeCountdown.value--
      if (emailCodeCountdown.value <= 0) stopEmailCodeCountdown()
    }, 1000)
  }

  onBeforeUnmount(stopEmailCodeCountdown)
  watch(
    () => registerForm.email,
    (email) => {
      if (emailCodeTarget.value && email.trim().toLowerCase() !== emailCodeTarget.value) {
        registerForm.emailCode = ''
      }
    }
  )

  const registerRules: FormRules = {
    email: [
      { required: true, message: '请输入邮箱', trigger: 'blur' },
      { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
    ],
    emailCode: [
      { required: true, message: '请输入邮箱验证码', trigger: 'blur' },
      { pattern: /^\d{6}$/, message: '请输入 6 位数字验证码', trigger: 'blur' }
    ],
    phone: [{ pattern: /^1\d{10}$/, message: '请输入有效的手机号', trigger: 'blur' }],
    nickname: [{ required: true, message: '请输入账号', trigger: 'blur' }],
    password: [
      { required: true, message: '请设置密码', trigger: 'blur' },
      { min: 6, message: '密码至少6位', trigger: 'blur' }
    ],
    confirmPassword: [
      { required: true, message: '请确认密码', trigger: 'blur' },
      {
        validator: (_rule: unknown, value: string, callback: (error?: Error) => void) => {
          if (value !== registerForm.password) callback(new Error('两次密码输入不一致'))
          else callback()
        },
        trigger: 'blur'
      }
    ]
  }

  const forgotForm = reactive({ email: '' })
  const forgotRules: FormRules = {
    email: [
      { required: true, message: '请输入注册邮箱', trigger: 'blur' },
      { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' }
    ]
  }

  function handleLogin() {
    loginFormRef.value?.validate(async (valid: boolean) => {
      if (!valid) return
      loading.value = true
      const account = loginForm.username.trim()
      const password = loginForm.password.trim()
      try {
        const { data } = await axios.post('/api/user-panel/login', { account, password })
        if (data.code === 200) {
          localStorage.setItem('user_panel_token', data.data.accessToken)
          localStorage.setItem('user_panel_info', JSON.stringify(data.data))
          ElMessage.success('登录成功')
          router.push('/user/dashboard')
        } else if (data.code === 409 && data.data?.converted) {
          authDialogVisible.value = false
          ElMessage.success(data.msg || '该账号已升级为代理，请前往代理端登录')
          router.push(data.data.loginPath || '/agent-panel/login?upgraded=1')
        } else {
          ElMessage.error(data.msg || '登录失败')
        }
      } catch {
        ElMessage.error('网络错误，请稍后重试')
      } finally {
        loading.value = false
      }
    })
  }

  function handleRequestEmailCode() {
    if (emailCodeCountdown.value > 0 || emailCodeSending.value) return
    registerFormRef.value?.validateField('email', (valid: boolean) => {
      if (!valid) return
      emailCaptchaPassing.value = false
      emailCaptchaVisible.value = true
      nextTick(() => emailCaptchaRef.value?.reset())
    })
  }

  async function handleEmailCaptchaPassed() {
    if (emailCodeSending.value) return
    const email = registerForm.email.trim().toLowerCase()
    if (!email) return

    emailCodeSending.value = true
    try {
      const { data } = await axios.post('/api/user-panel/register/email-code', { email })
      if (data.code === 200) {
        emailCodeTarget.value = email
        startEmailCodeCountdown()
        emailCaptchaVisible.value = false
        ElMessage.success(data.msg || '验证码已发送，请查收邮件')
      } else {
        ElMessage.error(data.msg || '验证码发送失败')
        emailCaptchaPassing.value = false
        nextTick(() => emailCaptchaRef.value?.reset())
      }
    } catch {
      ElMessage.error('网络错误，请稍后重试')
      emailCaptchaPassing.value = false
      nextTick(() => emailCaptchaRef.value?.reset())
    } finally {
      emailCodeSending.value = false
    }
  }

  function handleEmailCaptchaClosed() {
    emailCaptchaPassing.value = false
    nextTick(() => emailCaptchaRef.value?.reset())
  }

  function handleRegister() {
    if (!registrationEnabled.value) {
      ElMessage.warning('普通用户注册已关闭，请联系管理员')
      mode.value = 'login'
      return
    }

    registerFormRef.value?.validate(async (valid: boolean) => {
      if (!valid) return
      loading.value = true
      const email = registerForm.email.trim().toLowerCase()
      const emailCode = registerForm.emailCode.trim()
      const phone = registerForm.phone.trim()
      const nickname = registerForm.nickname.trim()
      const password = registerForm.password.trim()
      try {
        const { data } = await axios.post('/api/user-panel/register', {
          email,
          emailCode,
          phone,
          nickname,
          password
        })
        if (data.code === 200) {
          ElMessage.success('注册成功，正在登录...')
          const { data: loginData } = await axios.post('/api/user-panel/login', {
            account: email,
            password
          })
          if (loginData.code === 200) {
            localStorage.setItem('user_panel_token', loginData.data.accessToken)
            localStorage.setItem('user_panel_info', JSON.stringify(loginData.data))
            router.push('/user/dashboard')
          } else {
            mode.value = 'login'
          }
        } else {
          ElMessage.error(data.msg || '注册失败')
        }
      } catch {
        ElMessage.error('网络错误，请稍后重试')
      } finally {
        loading.value = false
      }
    })
  }

  function handleForgot() {
    forgotFormRef.value?.validate(async (valid: boolean) => {
      if (!valid) return
      loading.value = true
      const email = forgotForm.email.trim().toLowerCase()
      try {
        const { data } = await axios.post('/api/user-panel/forgot-password', { email })
        if (data.code === 200) {
          ElMessage.success(data.msg || '如果该邮箱已注册，重置链接将发送到您的邮箱')
          mode.value = 'login'
        } else {
          ElMessage.error(data.msg || '发送失败，请稍后重试')
        }
      } catch {
        ElMessage.error('网络错误，请稍后重试')
      } finally {
        loading.value = false
      }
    })
  }
</script>

<style scoped lang="scss">
  .license-home {
    --el-color-primary: #4d6bfe;
    --el-color-primary-light-3: #8297fe;
    --el-color-primary-light-5: #a6b5ff;
    --el-color-primary-light-7: #cad3ff;
    --el-color-primary-light-8: #dbe1ff;
    --el-color-primary-light-9: #edf0ff;
    --el-color-primary-dark-2: #3e56cb;

    min-height: 100vh;
    overflow: hidden;
    color: var(--el-text-color-primary);
    background: #fff;
  }

  .page-shell {
    width: min(1180px, calc(100% - 48px));
    margin: 0 auto;
  }

  .site-header {
    position: fixed;
    top: 0;
    right: 0;
    left: 0;
    z-index: 100;
    height: 76px;
    background: color-mix(in srgb, var(--el-bg-color) 96%, transparent);
    border-bottom: 1px solid var(--el-border-color-lighter);
    backdrop-filter: blur(8px);
  }

  .nav-shell {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: min(1240px, calc(100% - 40px));
    height: 100%;
    margin: 0 auto;
  }

  .brand-button {
    display: flex;
    gap: 11px;
    align-items: center;
    min-width: 220px;
    padding: 0;
    color: inherit;
    cursor: pointer;
    background: transparent;
    border: 0;
    text-align: left;
  }

  .brand-logo-wrap {
    display: grid;
    flex: 0 0 42px;
    width: 42px;
    height: 42px;
    overflow: hidden;
    place-items: center;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color);
    border-radius: 6px;
  }

  .brand-logo {
    width: 31px;
    height: 31px;
    object-fit: contain;
  }

  .brand-copy {
    display: flex;
    flex-direction: column;
    min-width: 0;

    strong {
      max-width: 190px;
      overflow: hidden;
      font-size: 17px;
      font-weight: 700;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    small {
      margin-top: 1px;
      font-size: 9px;
      color: var(--el-text-color-placeholder);
      letter-spacing: 1.5px;
      text-transform: uppercase;
    }
  }

  .main-nav {
    display: flex;
    gap: 5px;
    align-items: center;

    button {
      padding: 9px 14px;
      font-size: 14px;
      color: var(--el-text-color-regular);
      cursor: pointer;
      background: transparent;
      border: 0;
      border-radius: 9px;
      transition: 0.2s ease;

      &:hover {
        color: var(--el-color-primary);
        background: var(--el-fill-color-light);
      }
    }
  }

  .header-actions {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    min-width: 300px;

    :deep(.el-button) {
      border-radius: 10px;
    }

    :deep(.el-button + .el-button) {
      margin-left: 8px;
    }

    svg {
      width: 16px;
      height: 16px;
    }
  }

  .hero-section {
    position: relative;
    padding: 154px 0 112px;
    overflow: hidden;
    background-color: var(--el-bg-color);
    background-image: url('@/assets/images/home/license-service-bg.png');
    background-position: center;
    background-repeat: no-repeat;
    background-size: cover;

    &::before {
      position: absolute;
      inset: 0;
      pointer-events: none;
      content: '';
      background: linear-gradient(180deg, rgb(255 255 255 / 18%), rgb(255 255 255 / 4%));
    }
  }

  .hero-orb {
    display: none;
  }

  .hero-grid {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: minmax(0, 1.02fr) minmax(430px, 0.98fr);
    gap: 76px;
    align-items: center;
  }

  .hero-badge {
    display: inline-flex;
    gap: 9px;
    align-items: center;
    padding: 7px 11px;
    margin-bottom: 24px;
    font-size: 13px;
    color: var(--el-color-primary);
    background: var(--el-fill-color-light);
    border-left: 3px solid var(--el-color-primary);
    border-radius: 4px;
  }

  .badge-dot {
    width: 6px;
    height: 6px;
    background: var(--el-color-primary);
    border-radius: 50%;
  }

  .hero-content h1 {
    max-width: 680px;
    margin: 0;
    color: #111;
    font-size: clamp(36px, 4vw, 52px);
    font-weight: 700;
    line-height: 1.2;
    letter-spacing: -1.5px;
    animation: hero-title-fade 0.65s ease-out both;

    span {
      display: block;
      margin-top: 7px;
      color: #111;
      animation: hero-title-fade 0.65s 0.12s ease-out both;
    }
  }

  @keyframes hero-title-fade {
    from {
      opacity: 0;
      transform: translateY(8px);
    }

    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .hero-description {
    max-width: 620px;
    margin: 25px 0 32px;
    font-size: 17px;
    line-height: 1.9;
    color: var(--el-text-color-secondary);
  }

  .hero-actions {
    display: flex;
    gap: 12px;

    :deep(.el-button) {
      min-width: 150px;
      height: 48px;
      padding: 0 22px;
      font-weight: 600;
      border-radius: 12px;
    }

    svg {
      width: 17px;
      height: 17px;
    }
  }

  .hero-trust {
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
    margin-top: 34px;

    div {
      display: flex;
      gap: 7px;
      align-items: center;
      font-size: 12px;
      color: var(--el-text-color-secondary);
    }

    svg {
      width: 15px;
      height: 15px;
      color: var(--el-color-success);
    }
  }

  .hero-visual {
    position: relative;
    padding: 0;
    perspective: 1200px;
  }

  .visual-glow {
    display: none;
  }

  .dashboard-card {
    position: relative;
    z-index: 2;
    overflow: hidden;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color);
    border-radius: 10px;
    box-shadow: 0 12px 32px rgb(15 23 42 / 8%);
    transform-origin: center;
    animation: dashboard-card-enter 0.9s cubic-bezier(0.22, 1, 0.36, 1);
    transition:
      transform 0.35s cubic-bezier(0.22, 1, 0.36, 1),
      border-color 0.35s ease,
      box-shadow 0.35s ease;
    will-change: transform;

    &::after {
      position: absolute;
      top: -45%;
      bottom: -45%;
      left: -45%;
      z-index: 4;
      width: 28%;
      pointer-events: none;
      content: '';
      background: linear-gradient(90deg, transparent, rgb(255 255 255 / 42%), transparent);
      transform: translateX(-220%) rotate(12deg);
      transition: transform 0.8s ease;
    }
  }

  @media (hover: hover) {
    .dashboard-card:hover {
      border-color: #a6b5ff;
      box-shadow: 0 20px 42px rgb(77 107 254 / 18%);
      transform: translateY(-6px) rotateX(1.5deg) rotateY(-1.5deg) scale(1.008);

      &::after {
        transform: translateX(560%) rotate(12deg);
      }
    }
  }

  @keyframes dashboard-card-enter {
    0% {
      opacity: 0;
      transform: translateX(46px) rotateY(68deg) scale(0.94);
    }

    62% {
      opacity: 1;
      transform: translateX(-5px) rotateY(-6deg) scale(1.01);
    }

    82% {
      transform: translateX(2px) rotateY(2deg) scale(1);
    }

    100% {
      opacity: 1;
      transform: translateX(0) rotateY(0) scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .dashboard-card {
      animation: none;
      transition: none;

      &::after {
        display: none;
      }
    }
  }

  .dashboard-head {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    height: 54px;
    padding: 0 18px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    background: var(--el-fill-color-lighter);
    border-bottom: 1px solid var(--el-border-color-lighter);
  }

  .window-dots {
    display: flex;
    gap: 5px;

    i {
      width: 7px;
      height: 7px;
      background: var(--el-border-color);
      border-radius: 50%;

      &:first-child {
        background: #ff6b6b;
      }
      &:nth-child(2) {
        background: #f7b84b;
      }
      &:last-child {
        background: #4ecb71;
      }
    }
  }

  .online-pill {
    display: inline-flex;
    gap: 6px;
    align-items: center;
    justify-self: end;
    color: var(--el-color-success);

    i {
      width: 6px;
      height: 6px;
      background: currentcolor;
      border-radius: 50%;
      box-shadow: 0 0 0 4px rgb(103 194 58 / 10%);
    }
  }

  .dashboard-body {
    padding: 22px;
  }

  .summary-row {
    display: flex;
    align-items: center;
    padding: 17px;
    background: var(--el-fill-color-lighter);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;

    > div:nth-child(2) {
      display: flex;
      flex: 1;
      flex-direction: column;
      margin-left: 12px;
    }

    small {
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }

    strong {
      margin-top: 3px;
      font-size: 16px;
    }
  }

  .summary-icon {
    display: grid;
    width: 40px;
    height: 40px;
    color: var(--el-color-primary);
    place-items: center;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 6px;

    svg {
      width: 22px;
      height: 22px;
    }
  }

  .success-tag {
    padding: 5px 9px;
    font-size: 11px;
    color: var(--el-color-success);
    background: rgb(103 194 58 / 10%);
    border-radius: 999px;
  }

  .license-preview {
    padding: 18px;
    margin-top: 14px;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
  }

  .preview-title {
    display: flex;
    gap: 11px;
    align-items: center;
    padding-bottom: 15px;
    border-bottom: 1px solid var(--el-border-color-extra-light);

    > div {
      display: flex;
      flex-direction: column;
    }

    strong {
      font-size: 13px;
    }
    small {
      margin-top: 3px;
      font-size: 10px;
      color: var(--el-text-color-placeholder);
    }
  }

  .app-mark {
    display: grid;
    width: 35px;
    height: 35px;
    color: #fff;
    place-items: center;
    background: var(--el-color-primary);
    border-radius: 6px;

    svg {
      width: 18px;
      height: 18px;
    }
  }

  .preview-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 15px 20px;
    padding-top: 16px;

    div {
      display: flex;
      flex-direction: column;
    }
    small {
      font-size: 10px;
      color: var(--el-text-color-placeholder);
    }
    span {
      margin-top: 4px;
      font-size: 12px;
      font-weight: 600;
    }
  }

  .activity-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 13px 15px;
    margin-top: 14px;
    font-size: 10px;
    color: var(--el-text-color-secondary);
    background: var(--el-fill-color-lighter);
    border-radius: 12px;

    span {
      display: flex;
      gap: 7px;
      align-items: center;
    }
    i {
      width: 6px;
      height: 6px;
      background: var(--el-color-success);
      border-radius: 50%;
    }
    small {
      color: var(--el-text-color-placeholder);
    }
  }

  .floating-card {
    display: none;
  }

  .section-block {
    padding: 104px 0;
    scroll-margin-top: 74px;
  }

  .section-heading {
    max-width: 650px;
    margin-bottom: 44px;

    h2 {
      margin: 8px 0 13px;
      font-size: clamp(30px, 4vw, 42px);
      line-height: 1.25;
      letter-spacing: -1px;
    }

    p {
      font-size: 15px;
      line-height: 1.8;
      color: var(--el-text-color-secondary);
    }
  }

  .centered-heading {
    margin-right: auto;
    margin-left: auto;
    text-align: center;
  }

  .section-kicker {
    font-size: 11px;
    font-weight: 700;
    color: var(--el-color-primary);
    letter-spacing: 2px;
  }

  .query-section {
    background: #fff;
  }

  .flip-toggle-buttons {
    display: flex;
    gap: 10px;
    align-items: center;
    justify-content: center;
    margin-top: 20px;
  }

  .flip-toggle-btn {
    display: flex;
    gap: 7px;
    align-items: center;
    padding: 10px 20px;
    font-size: 14px;
    font-weight: 500;
    color: var(--el-text-color-regular);
    cursor: pointer;
    background: var(--el-fill-color-lighter);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 10px;
    transition: all 0.25s ease;

    svg {
      width: 16px;
      height: 16px;
      transition: transform 0.25s ease;
    }

    &:hover {
      color: var(--el-color-primary);
      background: var(--el-fill-color-light);
      border-color: var(--el-border-color);
    }

    &.active {
      color: #fff;
      background: var(--el-color-primary);
      border-color: var(--el-color-primary);
      box-shadow: 0 4px 12px rgb(77 107 254 / 20%);

      svg {
        transform: scale(1.1);
      }
    }
  }

  .flip-card-container {
    max-width: 920px;
    margin: 0 auto;
    perspective: 2000px;
  }

  .flip-card {
    position: relative;
    width: 100%;
    transition: transform 0.7s cubic-bezier(0.4, 0, 0.2, 1);
    transform-style: preserve-3d;
  }

  .flip-card-container.is-flipped .flip-card {
    transform: rotateY(180deg);
  }

  .flip-card-face {
    width: 100%;
    backface-visibility: hidden;
    -webkit-backface-visibility: hidden;
  }

  .flip-card-front {
    position: relative;
    z-index: 2;
  }

  .flip-card-back {
    position: absolute;
    top: 0;
    left: 0;
    z-index: 1;
    transform: rotateY(180deg);
  }

  .query-panel {
    max-width: 920px;
    padding: 30px;
    margin: 0 auto;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color);
    border-radius: 10px;
    box-shadow: 0 8px 24px rgb(15 23 42 / 5%);
  }

  .query-input-row {
    display: flex;
    gap: 12px;

    :deep(.el-input__wrapper) {
      min-height: 52px;
      padding: 1px 17px;
      background: var(--el-fill-color-lighter);
      border-radius: 12px;
      box-shadow: 0 0 0 1px var(--el-border-color-lighter) inset;
    }

    :deep(.el-input__wrapper.is-focus) {
      background: var(--el-bg-color);
      box-shadow: 0 0 0 1px var(--el-color-primary) inset;
    }

    :deep(.el-button) {
      flex: 0 0 138px;
      height: 52px;
      border-radius: 12px;
    }
  }

  .query-security-tip {
    display: flex;
    gap: 7px;
    align-items: center;
    margin-top: 13px;
    font-size: 12px;
    color: var(--el-text-color-placeholder);

    svg {
      flex: 0 0 auto;
      width: 15px;
      height: 15px;
    }
  }

  .query-results {
    padding-top: 26px;
    margin-top: 25px;
    border-top: 1px solid var(--el-border-color-extra-light);
  }

  .result-summary {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 18px;

    > div {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    > div > div {
      display: flex;
      flex-direction: column;
    }
    strong {
      font-size: 14px;
    }
    small {
      margin-top: 2px;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }
  }

  .result-icon {
    display: grid;
    width: 34px;
    height: 34px;
    color: var(--el-color-success);
    place-items: center;
    background: rgb(103 194 58 / 9%);
    border-radius: 10px;

    svg {
      width: 19px;
      height: 19px;
    }
  }

  .result-limit {
    font-size: 11px;
    color: var(--el-text-color-placeholder);
  }

  .license-result-grid {
    display: grid;
    gap: 13px;
  }

  .license-result-card {
    padding: 19px;
    background: var(--el-fill-color-extra-light);
    border: 1px solid var(--el-border-color-extra-light);
    border-radius: 15px;
  }

  .result-card-head {
    display: flex;
    align-items: center;
    padding-bottom: 15px;
    border-bottom: 1px solid var(--el-border-color-extra-light);
  }

  .result-app-icon {
    display: grid;
    width: 38px;
    height: 38px;
    color: var(--el-color-primary);
    place-items: center;
    background: rgb(64 158 255 / 10%);
    border-radius: 11px;

    svg {
      width: 20px;
      height: 20px;
    }
  }

  .result-app-name {
    display: flex;
    flex: 1;
    flex-direction: column;
    margin-left: 11px;

    strong {
      font-size: 14px;
    }
    span {
      margin-top: 2px;
      font-size: 11px;
      color: var(--el-text-color-placeholder);
    }
  }

  .status-badge {
    padding: 5px 10px;
    font-size: 11px;
    color: var(--el-text-color-secondary);
    background: var(--el-fill-color);
    border-radius: 999px;

    &.is-active {
      color: var(--el-color-success);
      background: rgb(103 194 58 / 10%);
    }
    &.is-expired {
      color: var(--el-color-warning);
      background: rgb(230 162 60 / 10%);
    }
    &.is-revoked {
      color: var(--el-color-danger);
      background: rgb(245 108 108 / 10%);
    }
  }

  .result-meta-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 18px;
    padding-top: 15px;

    div {
      display: flex;
      min-width: 0;
      flex-direction: column;
    }
    small {
      font-size: 10px;
      color: var(--el-text-color-placeholder);
    }
    strong {
      margin-top: 5px;
      overflow: hidden;
      font-size: 12px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .query-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 22px 10px 6px;
    text-align: center;

    > span {
      display: grid;
      width: 54px;
      height: 54px;
      margin-bottom: 13px;
      color: var(--el-text-color-placeholder);
      place-items: center;
      background: var(--el-fill-color-light);
      border-radius: 16px;
    }

    svg {
      width: 28px;
      height: 28px;
    }
    strong {
      font-size: 15px;
    }
    p {
      margin: 7px 0 15px;
      font-size: 12px;
      color: var(--el-text-color-placeholder);
    }
  }

  .features-section {
    background: var(--el-bg-color);
  }

  .feature-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 18px;
  }

  .feature-card {
    position: relative;
    min-height: 235px;
    padding: 26px;
    overflow: hidden;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;
    transition:
      border-color 0.2s ease,
      box-shadow 0.2s ease;

    &::after {
      display: none;
    }

    &:hover {
      border-color: var(--el-border-color);
      box-shadow: 0 8px 22px rgb(15 23 42 / 5%);
    }

    h3 {
      margin: 19px 0 10px;
      font-size: 18px;
    }
    p {
      min-height: 66px;
      font-size: 13px;
      line-height: 1.75;
      color: var(--el-text-color-secondary);
    }
  }

  .feature-card-icon {
    display: grid;
    width: 46px;
    height: 46px;
    color: var(--el-color-primary);
    place-items: center;
    background: var(--el-fill-color-light);
    border: 1px solid var(--el-border-color-extra-light);
    border-radius: 6px;

    svg {
      width: 23px;
      height: 23px;
    }
  }

  .feature-link {
    display: flex;
    gap: 5px;
    align-items: center;
    margin-top: 18px;
    font-size: 11px;
    font-weight: 600;
    color: var(--el-color-primary);

    svg {
      width: 13px;
      height: 13px;
    }
  }

  .workflow-section {
    background: #fff;
  }

  .workflow-shell {
    display: grid;
    grid-template-columns: 0.8fr 1.2fr;
    gap: 90px;
    align-items: center;
  }

  .workflow-heading {
    margin-bottom: 0;

    :deep(.el-button) {
      height: 46px;
      margin-top: 12px;
      border-radius: 11px;
    }
  }

  .workflow-list {
    position: relative;
    display: grid;
    gap: 14px;

    &::before {
      position: absolute;
      top: 66px;
      bottom: 66px;
      left: 48px;
      width: 1px;
      content: '';
      background: linear-gradient(var(--el-color-primary), var(--el-border-color-lighter));
    }
  }

  .workflow-item {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: 38px 48px 1fr;
    gap: 14px;
    align-items: center;
    padding: 20px;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 8px;

    h3 {
      margin: 0 0 5px;
      font-size: 15px;
    }
    p {
      margin: 0;
      font-size: 12px;
      line-height: 1.6;
      color: var(--el-text-color-secondary);
    }
  }

  .step-number {
    font-size: 11px;
    font-weight: 700;
    color: var(--el-color-primary);
    letter-spacing: 1px;
  }

  .step-icon {
    display: grid;
    width: 44px;
    height: 44px;
    color: var(--el-color-primary);
    place-items: center;
    background: var(--el-fill-color-light);
    border: 1px solid var(--el-border-color-extra-light);
    border-radius: 6px;

    svg {
      width: 21px;
      height: 21px;
    }
  }

  .cta-section {
    padding: 0 0 104px;
    background: #fff;
  }

  .cta-card {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 48px 52px;
    overflow: hidden;
    background: #4d6bfe;
    border: 1px solid #3e56cb;
    border-radius: 10px;

    h2 {
      margin: 8px 0 8px;
      font-size: 31px;
      color: #fff;
    }
    p {
      margin: 0;
      font-size: 14px;
      color: rgb(255 255 255 / 72%);
    }
    .section-kicker {
      color: rgb(255 255 255 / 68%);
    }
  }

  .cta-actions {
    display: flex;
    gap: 10px;
    margin-left: 30px;

    :deep(.el-button) {
      height: 46px;
      border-radius: 11px;
    }
    :deep(.el-button--primary) {
      color: var(--el-color-primary);
      background: #fff;
      border-color: #fff;
    }
    :deep(.el-button.is-plain) {
      color: #fff;
      background: rgb(255 255 255 / 8%);
      border-color: rgb(255 255 255 / 42%);
    }
  }

  .site-footer {
    padding: 34px 0;
    background: var(--el-bg-color);
    border-top: 1px solid var(--el-border-color-lighter);
  }

  .footer-shell {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .footer-brand {
    display: flex;
    gap: 10px;
    align-items: center;

    img {
      width: 34px;
      height: 34px;
      object-fit: contain;
    }
    div {
      display: flex;
      flex-direction: column;
    }
    strong {
      font-size: 13px;
    }
    span {
      margin-top: 2px;
      font-size: 10px;
      color: var(--el-text-color-placeholder);
    }
  }

  .footer-meta {
    display: flex;
    gap: 18px;
    font-size: 11px;
    color: var(--el-text-color-placeholder);

    a {
      color: inherit;
      text-decoration: none;
      &:hover {
        color: var(--el-color-primary);
      }
    }
  }

  .email-captcha-content p {
    margin-bottom: 18px;
    font-size: 14px;
    line-height: 1.7;
    color: var(--el-text-color-secondary);
    overflow-wrap: anywhere;
  }

  .email-captcha-slider {
    overflow: hidden;
    border-radius: 12px;
  }

  .announcement-content {
    max-height: min(55vh, 360px);
    padding: 4px 2px;
    overflow-y: auto;
    font-size: 14px;
    line-height: 1.8;
    color: var(--el-text-color-regular);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .announcement-footer {
    display: flex;
    gap: 16px;
    align-items: center;
    justify-content: space-between;
  }

  .card-fade-enter-active,
  .card-fade-leave-active {
    transition:
      opacity 0.2s ease,
      transform 0.2s ease;
  }

  .card-fade-enter-from {
    opacity: 0;
    transform: translateX(14px);
  }
  .card-fade-leave-to {
    opacity: 0;
    transform: translateX(-14px);
  }

  :global(.home-auth-dialog) {
    --el-color-primary: #4d6bfe;
    --el-color-primary-light-3: #8297fe;
    --el-color-primary-light-5: #a6b5ff;
    --el-color-primary-light-7: #cad3ff;
    --el-color-primary-light-8: #dbe1ff;
    --el-color-primary-light-9: #edf0ff;
    --el-color-primary-dark-2: #3e56cb;
    --el-bg-color: #fff;
    --el-fill-color-blank: #fff;
    --el-fill-color-lighter: #f7f8fa;
    --el-text-color-primary: #1f2329;
    --el-text-color-regular: #4e5969;
    --el-text-color-secondary: #86909c;
    --el-text-color-placeholder: #a8abb2;
    --el-border-color: #dcdfe6;
    --el-border-color-lighter: #e5e6eb;
    --el-border-color-extra-light: #f0f1f3;

    padding: 0 !important;
    overflow: hidden;
    background: #fff;
    border: 1px solid #e5e6eb;
    border-radius: 14px !important;
    box-shadow: 0 18px 55px rgb(15 23 42 / 16%);
  }

  :global(.home-auth-dialog .el-dialog__header) {
    padding: 20px 24px 16px !important;
    margin-right: 0;
    border-bottom: 1px solid var(--el-border-color-extra-light);
  }

  :global(.home-auth-dialog .el-dialog__headerbtn) {
    top: 16px;
    right: 16px;
  }

  :global(.home-auth-dialog .el-dialog__body) {
    max-height: min(72vh, 680px);
    padding: 26px 28px 28px !important;
    overflow-x: hidden;
    overflow-y: auto;
  }

  .auth-brand {
    display: flex;
    gap: 10px;
    align-items: center;

    img {
      width: 34px;
      height: 34px;
      object-fit: contain;
    }
    div {
      display: flex;
      flex-direction: column;
    }
    strong {
      max-width: 300px;
      overflow: hidden;
      font-size: 14px;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    span {
      margin-top: 2px;
      font-size: 10px;
      color: var(--el-text-color-placeholder);
    }
  }

  .auth-form-wrapper {
    width: 100%;
    min-width: 0;

    h2 {
      margin: 0;
      font-size: 24px;
      line-height: 1.3;
    }
  }

  .auth-subtitle {
    margin: 7px 0 24px;
    font-size: 13px;
    line-height: 1.6;
    color: var(--el-text-color-secondary);
  }

  .auth-form {
    width: 100%;

    :deep(.el-form-item) {
      width: 100%;
      margin-bottom: 16px;
    }

    :deep(.el-form-item__content) {
      min-width: 0;
    }

    :deep(.el-input) {
      width: 100%;
    }

    :deep(.el-input__wrapper) {
      min-height: 46px;
      padding: 1px 14px;
      background: var(--el-fill-color-lighter);
      border-radius: 8px;
      box-shadow: 0 0 0 1px var(--el-border-color-lighter) inset;
    }

    :deep(.el-input__wrapper.is-focus) {
      background: var(--el-bg-color);
      box-shadow: 0 0 0 1px var(--el-color-primary) inset;
    }
  }

  .form-options {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin: 2px 0 21px;
  }

  .form-grid-two {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 12px;

    :deep(.el-form-item) {
      min-width: 0;
    }
  }

  .email-code-row {
    display: flex;
    gap: 10px;
    width: 100%;

    :deep(.el-input) {
      min-width: 0;
    }
    :deep(.el-button) {
      flex: 0 0 118px;
      height: 46px;
      border-radius: 8px;
    }
  }

  .auth-submit {
    width: 100%;
    height: 46px;
    margin-top: 2px;
    font-weight: 600;
    border-radius: 8px;
  }

  .auth-switch {
    margin-top: 20px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    text-align: center;

    :deep(.el-link) {
      margin-left: 5px;
      font-size: 12px;
    }
  }

  @media (max-width: 1050px) {
    .main-nav {
      display: none;
    }
    .hero-grid {
      grid-template-columns: 1fr 420px;
      gap: 38px;
    }
    .hero-content h1 {
      font-size: 42px;
    }
    .floating-card-top {
      right: -8px;
    }
    .floating-card-bottom {
      left: -8px;
    }
  }

  @media (max-width: 860px) {
    .site-header {
      height: 68px;
    }
    .brand-button {
      min-width: 0;
    }
    .header-query {
      display: none;
    }
    .header-actions {
      min-width: 0;
    }
    .hero-section {
      padding: 126px 0 84px;
    }
    .hero-grid {
      grid-template-columns: 1fr;
    }
    .hero-content {
      text-align: center;
    }
    .hero-description {
      margin-right: auto;
      margin-left: auto;
    }
    .hero-actions,
    .hero-trust {
      justify-content: center;
    }
    .hero-visual {
      width: min(500px, 94%);
      margin: 10px auto 0;
    }
    .feature-grid {
      grid-template-columns: repeat(2, 1fr);
    }
    .workflow-shell {
      grid-template-columns: 1fr;
      gap: 44px;
    }
    .workflow-heading {
      max-width: 640px;
      text-align: center;
      margin: 0 auto;
    }
    .cta-card {
      align-items: flex-start;
      flex-direction: column;
    }
    .cta-actions {
      margin: 28px 0 0;
    }
    .result-meta-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 620px) {
    .page-shell {
      width: min(100% - 30px, 1180px);
    }
    .nav-shell {
      width: calc(100% - 24px);
    }
    .brand-logo-wrap {
      flex-basis: 36px;
      width: 36px;
      height: 36px;
    }
    .brand-logo {
      width: 27px;
      height: 27px;
    }
    .brand-copy strong {
      max-width: 118px;
      font-size: 14px;
    }
    .brand-copy small {
      display: none;
    }
    .header-actions :deep(.el-button) {
      padding: 8px 11px;
    }
    .header-actions :deep(.el-button + .el-button) {
      margin-left: 4px;
    }
    .hero-section {
      padding: 112px 0 70px;
    }
    .hero-content h1 {
      font-size: 32px;
      letter-spacing: -1px;
    }
    .hero-description {
      font-size: 14px;
    }
    .hero-actions {
      flex-direction: column;
    }
    .hero-actions :deep(.el-button) {
      width: 100%;
      margin-left: 0;
    }
    .hero-trust {
      display: grid;
      gap: 10px;
      text-align: left;
    }
    .dashboard-card {
      transform: none;
    }
    .floating-card {
      display: none;
    }
    .section-block {
      padding: 76px 0;
    }
    .section-heading {
      margin-bottom: 32px;
    }
    .section-heading h2 {
      font-size: 29px;
    }
    .flip-toggle-buttons {
      flex-wrap: wrap;
    }
    .flip-toggle-btn {
      flex: 1;
      min-width: 140px;
      justify-content: center;
    }
    .query-panel {
      padding: 20px 16px;
      border-radius: 17px;
    }
    .query-input-row {
      flex-direction: column;
    }
    .query-input-row :deep(.el-button) {
      flex-basis: 48px;
      width: 100%;
    }
    .query-security-tip {
      align-items: flex-start;
      line-height: 1.55;
    }
    .feature-grid {
      grid-template-columns: 1fr;
    }
    .feature-card {
      min-height: 0;
    }
    .workflow-item {
      grid-template-columns: 32px 40px 1fr;
      gap: 10px;
      padding: 16px;
    }
    .step-number {
      font-size: 10px;
    }
    .step-icon {
      width: 38px;
      height: 38px;
    }
    .step-icon svg {
      width: 18px;
      height: 18px;
    }
    .workflow-item h3 {
      font-size: 14px;
    }
    .workflow-item p {
      font-size: 11px;
    }
    .workflow-list::before {
      left: 40px;
    }
    .cta-card {
      padding: 36px 26px;
    }
    .cta-card h2 {
      font-size: 25px;
    }
    .cta-actions {
      flex-direction: column;
      margin-top: 22px;
      margin-left: 0;
    }
    .cta-actions :deep(.el-button) {
      width: 100%;
    }
    .footer-shell {
      flex-direction: column;
      gap: 18px;
      text-align: center;
    }
    .footer-meta {
      flex-direction: column;
      gap: 8px;
    }
    .result-meta-grid {
      grid-template-columns: 1fr;
      gap: 12px;
    }
    .form-grid-two {
      grid-template-columns: 1fr;
      gap: 0;
    }
    :global(.home-auth-dialog .el-dialog__body) {
      padding: 22px 20px 24px !important;
    }
    :global(.home-auth-dialog .el-dialog__header) {
      padding: 18px 20px 14px !important;
    }
  }

  @media (max-width: 430px) {
    .brand-copy strong {
      max-width: 105px;
    }
    .dashboard-body {
      padding: 15px;
    }
    .preview-grid,
    .result-meta-grid {
      grid-template-columns: 1fr 1fr;
      gap: 13px;
    }
    .result-card-head {
      align-items: flex-start;
    }
    .status-badge {
      margin-left: 7px;
    }
    .email-code-row {
      flex-direction: column;
    }
    .email-code-row :deep(.el-button) {
      flex-basis: 42px;
      width: 100%;
    }
  }
</style>
