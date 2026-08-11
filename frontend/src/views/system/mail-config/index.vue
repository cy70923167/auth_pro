<template>
  <div class="mail-config-page">
    <ElCard v-loading="loading" class="config-card" shadow="never">
      <template #header>
        <div class="card-header">
          <div>
            <h2>邮件配置</h2>
            <p>配置 SMTP 发信服务、多模板通知、HTML 内容和到期提醒规则</p>
          </div>
          <div class="header-actions">
            <ElButton :loading="testing" @click="handleTestSend">发送测试</ElButton>
            <ElButton type="primary" :loading="saving" @click="handleSave">保存配置</ElButton>
          </div>
        </div>
      </template>

      <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" class="config-form">
        <ElRow :gutter="24">
          <ElCol :xs="24" :lg="14">
            <div class="section-card">
              <div class="section-title">发信服务</div>
              <ElRow :gutter="16">
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="邮箱类型" prop="provider">
                    <ElSelect v-model="form.provider" @change="applyProviderDefaults">
                      <ElOption label="QQ邮箱" value="qq" />
                      <ElOption label="阿里云邮箱" value="aliyun" />
                      <ElOption label="自定义SMTP" value="custom" />
                    </ElSelect>
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="加密方式" prop="smtpSecure">
                    <ElSelect v-model="form.smtpSecure">
                      <ElOption label="SSL" value="ssl" />
                      <ElOption label="STARTTLS" value="starttls" />
                      <ElOption label="不加密" value="none" />
                    </ElSelect>
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="16">
                  <ElFormItem label="SMTP地址" prop="smtpHost">
                    <ElInput v-model.trim="form.smtpHost" placeholder="例如 smtp.qq.com" />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="8">
                  <ElFormItem label="SMTP端口" prop="smtpPort">
                    <ElInputNumber
                      v-model="form.smtpPort"
                      :min="1"
                      :max="65535"
                      class="full-input"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="发信账号" prop="smtpUsername">
                    <ElInput v-model.trim="form.smtpUsername" placeholder="邮箱账号" />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="密码/授权码" prop="smtpPassword">
                    <ElInput
                      v-model="form.smtpPassword"
                      type="password"
                      show-password
                      :placeholder="form.passwordSet ? '已设置，留空不修改' : '请输入密码或授权码'"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="发件人邮箱" prop="smtpFromEmail">
                    <ElInput v-model.trim="form.smtpFromEmail" placeholder="展示给收件人的邮箱" />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="12">
                  <ElFormItem label="发件人名称" prop="smtpFromName">
                    <ElInput v-model.trim="form.smtpFromName" placeholder="例如 授权管理系统" />
                  </ElFormItem>
                </ElCol>
              </ElRow>
            </div>

            <div class="section-card">
              <div class="section-title">通知规则</div>
              <ElRow :gutter="16">
                <ElCol :xs="24" :md="8">
                  <ElFormItem label="购买成功邮件">
                    <ElSwitch
                      v-model="form.enabledPurchaseSuccess"
                      active-text="启用"
                      inactive-text="关闭"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="8">
                  <ElFormItem label="到期提醒邮件">
                    <ElSwitch
                      v-model="form.enabledExpireReminder"
                      active-text="启用"
                      inactive-text="关闭"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24" :md="8">
                  <ElFormItem label="后台开通邮件">
                    <ElSwitch
                      v-model="form.enabledLicenseOpened"
                      active-text="启用"
                      inactive-text="关闭"
                    />
                  </ElFormItem>
                </ElCol>
                <ElCol :xs="24">
                  <ElFormItem label="提前提醒天数" prop="expireRemindDays">
                    <ElSelect
                      v-model="remindDayList"
                      multiple
                      filterable
                      allow-create
                      default-first-option
                      placeholder="选择或输入提醒天数"
                      @change="syncRemindDays"
                    >
                      <ElOption
                        v-for="day in defaultRemindDays"
                        :key="day"
                        :label="`${day} 天`"
                        :value="String(day)"
                      />
                    </ElSelect>
                  </ElFormItem>
                </ElCol>
              </ElRow>
            </div>

            <div class="section-card template-section-card">
              <div class="section-title template-section-title">
                <div>
                  邮件内容自定义
                  <span>分别配置购买成功、到期提醒、后台开通三类邮件内容</span>
                </div>
              </div>
              <ElAlert
                title="这里可以自定义邮件标题、正文、内容格式，并插入授权码、到期时间等变量。"
                type="success"
                show-icon
                :closable="false"
                class="template-alert"
              />
              <ElTabs v-model="activeTemplate">
                <ElTabPane label="购买成功" name="purchase">
                  <div class="template-editor">
                    <ElFormItem label="邮件标题" prop="purchaseSubject">
                      <ElInput v-model="form.purchaseSubject" />
                    </ElFormItem>
                    <ElFormItem label="内容类型" class="content-type-form-item">
                      <ElRadioGroup
                        v-model="form.purchaseContentType"
                        class="content-type-selector"
                        :disabled="contentTypeSaving !== null"
                        @change="handleContentTypeChange('purchase')"
                      >
                        <ElRadioButton label="text">
                          <span class="content-type-option">
                            <span class="content-type-icon">TXT</span>
                            <span>
                              <strong>普通文本</strong>
                              <small>纯文字内容，兼容性更好</small>
                            </span>
                          </span>
                        </ElRadioButton>
                        <ElRadioButton label="html">
                          <span class="content-type-option">
                            <span class="content-type-icon">&lt;/&gt;</span>
                            <span>
                              <strong>HTML 源码</strong>
                              <small>使用排版模板展示邮件</small>
                            </span>
                          </span>
                        </ElRadioButton>
                      </ElRadioGroup>
                      <div
                        class="content-type-status"
                        :class="{ 'is-saving': contentTypeSaving === 'purchase' }"
                      >
                        {{
                          contentTypeSaving === 'purchase'
                            ? '正在应用并自动保存…'
                            : '选择后自动保存，HTML 源码会自动载入默认模板'
                        }}
                      </div>
                    </ElFormItem>
                    <div class="editor-toolbar">
                      <ElTag
                        v-for="item in variables"
                        :key="item.key"
                        effect="plain"
                        @click="appendTemplateVariable('purchase', item.key)"
                      >
                        {{ item.key }}
                      </ElTag>
                    </div>
                    <ElFormItem label="邮件内容自定义" prop="purchaseContent">
                      <ElInput v-model="form.purchaseContent" type="textarea" :rows="12" />
                    </ElFormItem>
                    <div class="preview-box">
                      <div class="preview-title">预览</div>
                      <div
                        class="preview-content"
                        v-html="renderPreview(form.purchaseContent, form.purchaseContentType)"
                      ></div>
                    </div>
                  </div>
                </ElTabPane>
                <ElTabPane label="到期提醒" name="expire">
                  <div class="template-editor">
                    <ElFormItem label="邮件标题" prop="expireSubject">
                      <ElInput v-model="form.expireSubject" />
                    </ElFormItem>
                    <ElFormItem label="内容类型" class="content-type-form-item">
                      <ElRadioGroup
                        v-model="form.expireContentType"
                        class="content-type-selector"
                        :disabled="contentTypeSaving !== null"
                        @change="handleContentTypeChange('expire')"
                      >
                        <ElRadioButton label="text">
                          <span class="content-type-option">
                            <span class="content-type-icon">TXT</span>
                            <span>
                              <strong>普通文本</strong>
                              <small>纯文字内容，兼容性更好</small>
                            </span>
                          </span>
                        </ElRadioButton>
                        <ElRadioButton label="html">
                          <span class="content-type-option">
                            <span class="content-type-icon">&lt;/&gt;</span>
                            <span>
                              <strong>HTML 源码</strong>
                              <small>使用排版模板展示邮件</small>
                            </span>
                          </span>
                        </ElRadioButton>
                      </ElRadioGroup>
                      <div
                        class="content-type-status"
                        :class="{ 'is-saving': contentTypeSaving === 'expire' }"
                      >
                        {{
                          contentTypeSaving === 'expire'
                            ? '正在应用并自动保存…'
                            : '选择后自动保存，HTML 源码会自动载入默认模板'
                        }}
                      </div>
                    </ElFormItem>
                    <div class="editor-toolbar">
                      <ElTag
                        v-for="item in variables"
                        :key="item.key"
                        effect="plain"
                        @click="appendTemplateVariable('expire', item.key)"
                      >
                        {{ item.key }}
                      </ElTag>
                    </div>
                    <ElFormItem label="邮件内容自定义" prop="expireContent">
                      <ElInput v-model="form.expireContent" type="textarea" :rows="12" />
                    </ElFormItem>
                    <div class="preview-box">
                      <div class="preview-title">预览</div>
                      <div
                        class="preview-content"
                        v-html="renderPreview(form.expireContent, form.expireContentType)"
                      ></div>
                    </div>
                  </div>
                </ElTabPane>
                <ElTabPane label="后台开通" name="opened">
                  <div class="template-editor">
                    <ElFormItem label="邮件标题" prop="openedSubject">
                      <ElInput v-model="form.openedSubject" />
                    </ElFormItem>
                    <ElFormItem label="内容类型" class="content-type-form-item">
                      <ElRadioGroup
                        v-model="form.openedContentType"
                        class="content-type-selector"
                        :disabled="contentTypeSaving !== null"
                        @change="handleContentTypeChange('opened')"
                      >
                        <ElRadioButton label="text">
                          <span class="content-type-option">
                            <span class="content-type-icon">TXT</span>
                            <span>
                              <strong>普通文本</strong>
                              <small>纯文字内容，兼容性更好</small>
                            </span>
                          </span>
                        </ElRadioButton>
                        <ElRadioButton label="html">
                          <span class="content-type-option">
                            <span class="content-type-icon">&lt;/&gt;</span>
                            <span>
                              <strong>HTML 源码</strong>
                              <small>使用排版模板展示邮件</small>
                            </span>
                          </span>
                        </ElRadioButton>
                      </ElRadioGroup>
                      <div
                        class="content-type-status"
                        :class="{ 'is-saving': contentTypeSaving === 'opened' }"
                      >
                        {{
                          contentTypeSaving === 'opened'
                            ? '正在应用并自动保存…'
                            : '选择后自动保存，HTML 源码会自动载入默认模板'
                        }}
                      </div>
                    </ElFormItem>
                    <div class="editor-toolbar">
                      <ElTag
                        v-for="item in variables"
                        :key="item.key"
                        effect="plain"
                        @click="appendTemplateVariable('opened', item.key)"
                      >
                        {{ item.key }}
                      </ElTag>
                    </div>
                    <ElFormItem label="自定义邮件内容" prop="openedContent">
                      <ElInput v-model="form.openedContent" type="textarea" :rows="12" />
                    </ElFormItem>
                    <div class="preview-box">
                      <div class="preview-title">预览</div>
                      <div
                        class="preview-content"
                        v-html="renderPreview(form.openedContent, form.openedContentType)"
                      ></div>
                    </div>
                  </div>
                </ElTabPane>
              </ElTabs>
            </div>
          </ElCol>

          <ElCol :xs="24" :lg="10">
            <div class="side-panel">
              <div class="section-title">测试发送</div>
              <ElFormItem label="测试收件邮箱">
                <ElInput v-model.trim="testRecipient" placeholder="请输入用于测试的邮箱地址" />
              </ElFormItem>
              <ElAlert
                title="QQ邮箱需要使用授权码；阿里云邮箱不同产品的 SMTP 地址可能不同，可按实际邮箱服务修改。"
                type="info"
                show-icon
                :closable="false"
              />
            </div>

            <div class="side-panel">
              <div class="section-title">可用变量</div>
              <div class="variable-list">
                <ElTag
                  v-for="item in variables"
                  :key="item.key"
                  effect="plain"
                  @click="copyVariable(item.key)"
                >
                  {{ item.key }} {{ item.label }}
                </ElTag>
              </div>
            </div>

            <div class="side-panel">
              <div class="section-title">发送范围</div>
              <ElDescriptions :column="1" border>
                <ElDescriptionsItem label="购买成功"
                  >用户自助购买、代理商开通授权</ElDescriptionsItem
                >
                <ElDescriptionsItem label="后台开通">管理员手动创建授权</ElDescriptionsItem>
                <ElDescriptionsItem label="到期提醒">按授权到期日匹配提醒天数</ElDescriptionsItem>
                <ElDescriptionsItem label="收件地址">用户表或代理商表注册邮箱</ElDescriptionsItem>
              </ElDescriptions>
            </div>
          </ElCol>
        </ElRow>
      </ElForm>
    </ElCard>
    <ElBacktop target="#app-main" :right="32" :bottom="32" />
  </div>
</template>

<script setup lang="ts">
  import type { FormInstance, FormRules } from 'element-plus'
  import {
    fetchMailConfig,
    fetchTestMailConfig,
    fetchUpdateMailConfig,
    fetchUpdateMailContentType,
    MailConfigData
  } from '@/api/system-manage'

  defineOptions({ name: 'MailConfig' })

  const formRef = ref<FormInstance>()
  const loading = ref(false)
  const saving = ref(false)
  const testing = ref(false)
  const contentTypeSaving = ref<MailTemplateType | null>(null)
  const activeTemplate = ref('purchase')
  const testRecipient = ref('')
  const remindDayList = ref<string[]>(['7', '3', '1'])
  const defaultRemindDays = [30, 15, 7, 3, 1]

  type MailTemplateType = 'purchase' | 'expire' | 'opened'

  const mailTemplateTypes: MailTemplateType[] = ['purchase', 'expire', 'opened']

  const form = reactive<MailConfigData>({
    provider: 'qq',
    smtpHost: 'smtp.qq.com',
    smtpPort: 465,
    smtpSecure: 'ssl',
    smtpUsername: '',
    smtpPassword: '',
    passwordSet: false,
    smtpFromEmail: '',
    smtpFromName: '授权管理系统',
    enabledPurchaseSuccess: true,
    enabledExpireReminder: true,
    enabledLicenseOpened: true,
    expireRemindDays: '7,3,1',
    purchaseSubject: '{{appName}} 开通成功通知',
    purchaseContent:
      '您好，{{ownerName}}：\n\n您购买的应用 {{appName}} 已开通成功。\n\n开通时间：{{openedAt}}\n到期时间：{{expiresAt}}\n授权码：{{licenseKey}}\n\n请妥善保存相关信息。',
    purchaseContentType: 'text',
    expireSubject: '{{appName}} 即将到期提醒',
    expireContent:
      '您好，{{ownerName}}：\n\n您的应用 {{appName}} 将于 {{expiresAt}} 到期，剩余 {{daysLeft}} 天。\n\n请及时续费，以免影响正常使用。',
    expireContentType: 'text',
    openedSubject: '{{appName}} 授权开通通知',
    openedContent:
      '您好，{{ownerName}}：\n\n管理员已为您开通应用 {{appName}} 的授权。\n\n开通时间：{{openedAt}}\n到期时间：{{expiresAt}}\n授权码：{{licenseKey}}\n\n请妥善保存相关信息。',
    openedContentType: 'text'
  })

  const defaultHtmlTemplates: Record<MailTemplateType, string> = {
    purchase: `<div style="margin:0;padding:32px 16px;background:#f3f4f6;font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2937;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;box-shadow:0 10px 30px rgba(15,23,42,.08);">
    <tr>
      <td style="padding:30px 36px;background:#2563eb;color:#ffffff;">
        <div style="font-size:14px;opacity:.85;">{{siteName}}</div>
        <h1 style="margin:10px 0 0;font-size:26px;line-height:1.4;">购买成功，授权已开通</h1>
      </td>
    </tr>
    <tr>
      <td style="padding:34px 36px;">
        <p style="margin:0 0 16px;font-size:16px;line-height:1.8;">您好，{{ownerName}}：</p>
        <p style="margin:0 0 24px;font-size:15px;line-height:1.8;color:#4b5563;">您购买的应用 <strong style="color:#111827;">{{appName}}</strong> 已成功开通，授权信息如下：</p>
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;background:#f8fafc;border-radius:12px;">
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">授权编号</td><td style="padding:14px 18px;text-align:right;font-weight:600;border-bottom:1px solid #e5e7eb;">{{licenseNo}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">授权码</td><td style="padding:14px 18px;text-align:right;font-family:monospace;font-weight:700;color:#2563eb;border-bottom:1px solid #e5e7eb;">{{licenseKey}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">开通时间</td><td style="padding:14px 18px;text-align:right;border-bottom:1px solid #e5e7eb;">{{openedAt}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;">到期时间</td><td style="padding:14px 18px;text-align:right;">{{expiresAt}}</td></tr>
        </table>
        <p style="margin:24px 0 0;padding:14px 16px;background:#eff6ff;border-left:4px solid #2563eb;border-radius:8px;font-size:14px;line-height:1.7;color:#1e40af;">请妥善保存授权信息，不要将授权码泄露给他人。</p>
      </td>
    </tr>
    <tr><td style="padding:20px 36px;background:#f8fafc;text-align:center;font-size:12px;color:#94a3b8;">此邮件由 {{siteName}} 自动发送，请勿直接回复。</td></tr>
  </table>
</div>`,
    expire: `<div style="margin:0;padding:32px 16px;background:#fff7ed;font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2937;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;box-shadow:0 10px 30px rgba(124,45,18,.10);">
    <tr>
      <td style="padding:30px 36px;background:#ea580c;color:#ffffff;">
        <div style="font-size:14px;opacity:.85;">{{siteName}}</div>
        <h1 style="margin:10px 0 0;font-size:26px;line-height:1.4;">授权即将到期</h1>
      </td>
    </tr>
    <tr>
      <td style="padding:34px 36px;">
        <p style="margin:0 0 16px;font-size:16px;line-height:1.8;">您好，{{ownerName}}：</p>
        <p style="margin:0 0 22px;font-size:15px;line-height:1.8;color:#4b5563;">您的应用 <strong style="color:#111827;">{{appName}}</strong> 授权即将到期，请及时处理。</p>
        <div style="margin-bottom:24px;padding:22px;text-align:center;background:#fff7ed;border:1px solid #fed7aa;border-radius:12px;">
          <div style="font-size:13px;color:#9a3412;">剩余有效期</div>
          <div style="margin-top:8px;font-size:34px;font-weight:700;color:#ea580c;">{{daysLeft}} 天</div>
        </div>
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;background:#f8fafc;border-radius:12px;">
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">授权编号</td><td style="padding:14px 18px;text-align:right;font-weight:600;border-bottom:1px solid #e5e7eb;">{{licenseNo}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;">到期时间</td><td style="padding:14px 18px;text-align:right;font-weight:600;color:#c2410c;">{{expiresAt}}</td></tr>
        </table>
        <p style="margin:24px 0 0;font-size:14px;line-height:1.8;color:#64748b;">请及时续费，以免授权到期后影响应用正常使用。</p>
      </td>
    </tr>
    <tr><td style="padding:20px 36px;background:#f8fafc;text-align:center;font-size:12px;color:#94a3b8;">此邮件由 {{siteName}} 自动发送，请勿直接回复。</td></tr>
  </table>
</div>`,
    opened: `<div style="margin:0;padding:32px 16px;background:#f0fdf4;font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2937;">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:640px;margin:0 auto;background:#ffffff;border-radius:16px;overflow:hidden;box-shadow:0 10px 30px rgba(20,83,45,.10);">
    <tr>
      <td style="padding:30px 36px;background:#16a34a;color:#ffffff;">
        <div style="font-size:14px;opacity:.85;">{{siteName}}</div>
        <h1 style="margin:10px 0 0;font-size:26px;line-height:1.4;">授权开通通知</h1>
      </td>
    </tr>
    <tr>
      <td style="padding:34px 36px;">
        <p style="margin:0 0 16px;font-size:16px;line-height:1.8;">您好，{{ownerName}}：</p>
        <p style="margin:0 0 24px;font-size:15px;line-height:1.8;color:#4b5563;">管理员已为您开通应用 <strong style="color:#111827;">{{appName}}</strong> 的授权。</p>
        <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="border-collapse:collapse;background:#f8fafc;border-radius:12px;">
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">授权编号</td><td style="padding:14px 18px;text-align:right;font-weight:600;border-bottom:1px solid #e5e7eb;">{{licenseNo}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">授权码</td><td style="padding:14px 18px;text-align:right;font-family:monospace;font-weight:700;color:#16a34a;border-bottom:1px solid #e5e7eb;">{{licenseKey}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;border-bottom:1px solid #e5e7eb;">开通时间</td><td style="padding:14px 18px;text-align:right;border-bottom:1px solid #e5e7eb;">{{openedAt}}</td></tr>
          <tr><td style="padding:14px 18px;color:#64748b;">到期时间</td><td style="padding:14px 18px;text-align:right;">{{expiresAt}}</td></tr>
        </table>
        <p style="margin:24px 0 0;padding:14px 16px;background:#f0fdf4;border-left:4px solid #16a34a;border-radius:8px;font-size:14px;line-height:1.7;color:#166534;">授权已生效，请妥善保存以上信息。</p>
      </td>
    </tr>
    <tr><td style="padding:20px 36px;background:#f8fafc;text-align:center;font-size:12px;color:#94a3b8;">此邮件由 {{siteName}} 自动发送，请勿直接回复。</td></tr>
  </table>
</div>`
  }

  const templateFields = {
    purchase: { content: 'purchaseContent', contentType: 'purchaseContentType' },
    expire: { content: 'expireContent', contentType: 'expireContentType' },
    opened: { content: 'openedContent', contentType: 'openedContentType' }
  } as const

  const variables = [
    { key: '{{appName}}', label: '应用名称' },
    { key: '{{licenseKey}}', label: '授权码' },
    { key: '{{licenseNo}}', label: '授权编号' },
    { key: '{{openedAt}}', label: '开通时间' },
    { key: '{{expiresAt}}', label: '到期时间' },
    { key: '{{daysLeft}}', label: '剩余天数' },
    { key: '{{ownerName}}', label: '用户/代理商' },
    { key: '{{siteName}}', label: '网站名称' }
  ]

  const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  const rules: FormRules<MailConfigData> = {
    smtpHost: [{ required: true, message: '请输入SMTP地址', trigger: 'blur' }],
    smtpPort: [{ required: true, message: '请输入SMTP端口', trigger: 'blur' }],
    smtpUsername: [{ required: true, message: '请输入发信账号', trigger: 'blur' }],
    smtpFromEmail: [
      { required: true, message: '请输入发件人邮箱', trigger: 'blur' },
      { pattern: emailPattern, message: '邮箱格式不正确', trigger: 'blur' }
    ],
    smtpFromName: [{ required: true, message: '请输入发件人名称', trigger: 'blur' }],
    expireRemindDays: [
      { required: true, message: '请选择提醒天数', trigger: 'change' },
      { pattern: /^\d+(,\d+)*$/, message: '提醒天数格式不正确', trigger: 'change' }
    ],
    purchaseSubject: [{ required: true, message: '请输入购买成功邮件标题', trigger: 'blur' }],
    purchaseContent: [{ required: true, message: '请输入购买成功邮件内容', trigger: 'blur' }],
    expireSubject: [{ required: true, message: '请输入到期提醒邮件标题', trigger: 'blur' }],
    expireContent: [{ required: true, message: '请输入到期提醒邮件内容', trigger: 'blur' }],
    openedSubject: [{ required: true, message: '请输入后台开通邮件标题', trigger: 'blur' }],
    openedContent: [{ required: true, message: '请输入后台开通邮件内容', trigger: 'blur' }]
  }

  onMounted(() => {
    loadConfig()
  })

  const loadConfig = async () => {
    loading.value = true
    try {
      const data = await fetchMailConfig()
      Object.assign(form, data, { smtpPassword: '' })
      mailTemplateTypes.forEach((template) => {
        applyDefaultHtmlTemplate(template)
      })
      remindDayList.value = String(form.expireRemindDays || '7,3,1')
        .split(',')
        .map((item) => item.trim())
        .filter(Boolean)
    } finally {
      loading.value = false
    }
  }

  const syncRemindDays = () => {
    const days = remindDayList.value
      .map((item) => Number(item))
      .filter((item) => Number.isInteger(item) && item > 0 && item <= 365)
    remindDayList.value = Array.from(new Set(days)).map(String)
    form.expireRemindDays = remindDayList.value.join(',')
  }

  const applyProviderDefaults = () => {
    if (form.provider === 'qq') {
      form.smtpHost = 'smtp.qq.com'
      form.smtpPort = 465
      form.smtpSecure = 'ssl'
    } else if (form.provider === 'aliyun') {
      form.smtpHost = 'smtp.qiye.aliyun.com'
      form.smtpPort = 465
      form.smtpSecure = 'ssl'
    }
  }

  const handleSave = async () => {
    if (!formRef.value) return
    syncRemindDays()
    await formRef.value.validate()
    saving.value = true
    try {
      const data = await fetchUpdateMailConfig({ ...form })
      Object.assign(form, data, { smtpPassword: '' })
      ElMessage.success('邮件配置已保存')
    } finally {
      saving.value = false
    }
  }

  const handleTestSend = async () => {
    if (!emailPattern.test(testRecipient.value)) {
      ElMessage.error('请输入正确的测试邮箱')
      return
    }
    testing.value = true
    try {
      await fetchTestMailConfig(testRecipient.value)
    } finally {
      testing.value = false
    }
  }

  const copyVariable = async (value: string) => {
    await navigator.clipboard?.writeText(value)
    ElMessage.success(`已复制 ${value}`)
  }

  const applyDefaultHtmlTemplate = (template: MailTemplateType) => {
    const fields = templateFields[template]
    if (form[fields.contentType] !== 'html') return

    const currentContent = form[fields.content].trim()
    const hasHtmlMarkup = /<([a-z][\w-]*)(?:\s[^>]*)?>/i.test(currentContent)
    if (hasHtmlMarkup) return

    form[fields.content] = defaultHtmlTemplates[template]
  }

  const handleContentTypeChange = async (template: MailTemplateType) => {
    const fields = templateFields[template]
    const previousContent = form[fields.content]
    const previousContentType = form[fields.contentType] === 'html' ? 'text' : 'html'

    applyDefaultHtmlTemplate(template)
    contentTypeSaving.value = template
    try {
      await fetchUpdateMailContentType({
        template,
        contentType: form[fields.contentType],
        content: form[fields.content]
      })
      ElMessage.success('内容类型已自动保存')
    } catch {
      form[fields.contentType] = previousContentType
      form[fields.content] = previousContent
      ElMessage.error('内容类型自动保存失败，已恢复原设置')
    } finally {
      contentTypeSaving.value = null
    }
  }

  const appendTemplateVariable = (template: MailTemplateType, value: string) => {
    if (template === 'purchase') {
      form.purchaseContent += value
    } else if (template === 'expire') {
      form.expireContent += value
    } else {
      form.openedContent += value
    }
  }

  const renderPreview = (content: string, contentType: 'text' | 'html') => {
    if (contentType === 'html') return content
    return content.replace(/\n/g, '<br />')
  }
</script>

<style scoped lang="scss">
  .mail-config-page {
    .config-card {
      min-height: 100%;
      border: 0;
    }

    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;

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

    .header-actions {
      display: flex;
      gap: 10px;
      align-items: center;
    }

    .config-form {
      :deep(.el-select),
      :deep(.el-input-number) {
        width: 100%;
      }
    }

    .section-card,
    .side-panel {
      padding: 20px;
      margin-bottom: 18px;
      border: 1px solid var(--art-border-color);
      border-radius: 14px;
      background: var(--art-main-bg-color);
    }

    .section-title {
      margin-bottom: 18px;
      font-size: 16px;
      font-weight: 600;
      color: var(--art-gray-900);

      span {
        display: block;
        margin-top: 6px;
        font-size: 13px;
        font-weight: 400;
        color: var(--art-gray-600);
      }
    }

    .template-section-card {
      scroll-margin-top: 24px;
      border-color: var(--el-color-primary-light-5);
      background: linear-gradient(
        180deg,
        var(--el-color-primary-light-9),
        var(--art-main-bg-color) 88px
      );
    }

    .template-section-title {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
    }

    .template-alert {
      margin-bottom: 16px;
    }

    .content-type-form-item {
      :deep(.el-form-item__content) {
        display: block;
      }
    }

    .content-type-selector {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
      width: 100%;

      :deep(.el-radio-button) {
        width: 100%;
      }

      :deep(.el-radio-button__inner) {
        display: block;
        width: 100%;
        min-height: 76px;
        padding: 14px 16px;
        white-space: normal;
        text-align: left;
        color: var(--art-gray-700);
        border: 1px solid var(--art-border-color) !important;
        border-radius: 12px !important;
        background: var(--art-main-bg-color);
        box-shadow: none !important;
        transition:
          border-color 0.2s ease,
          background-color 0.2s ease,
          box-shadow 0.2s ease;
      }

      :deep(.el-radio-button__inner:hover) {
        color: var(--el-color-primary);
        border-color: var(--el-color-primary-light-5) !important;
      }

      :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
        color: var(--el-color-primary);
        border-color: var(--el-color-primary) !important;
        background: var(--el-color-primary-light-9);
        box-shadow: 0 0 0 2px var(--el-color-primary-light-8) !important;
      }
    }

    .content-type-option {
      display: flex;
      align-items: center;
      gap: 12px;

      strong,
      small {
        display: block;
      }

      strong {
        margin-bottom: 5px;
        font-size: 14px;
        line-height: 1.2;
      }

      small {
        font-size: 12px;
        line-height: 1.4;
        color: var(--art-gray-500);
      }
    }

    .content-type-icon {
      display: inline-flex;
      flex: 0 0 42px;
      align-items: center;
      justify-content: center;
      height: 36px;
      font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
      font-size: 12px;
      font-weight: 700;
      border-radius: 9px;
      background: var(--el-fill-color-light);
    }

    .content-type-status {
      display: flex;
      align-items: center;
      gap: 7px;
      margin-top: 10px;
      font-size: 12px;
      color: var(--art-gray-500);

      &::before {
        width: 6px;
        height: 6px;
        content: '';
        border-radius: 50%;
        background: var(--el-color-success);
      }

      &.is-saving {
        color: var(--el-color-primary);

        &::before {
          background: var(--el-color-primary);
          animation: mail-content-saving 1s ease-in-out infinite;
        }
      }
    }

    .variable-list,
    .editor-toolbar {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-bottom: 14px;

      .el-tag {
        cursor: pointer;
      }
    }

    .preview-box {
      padding: 14px;
      border: 1px dashed var(--art-border-color);
      border-radius: 12px;
      background: var(--art-bg-color);
    }

    .preview-title {
      margin-bottom: 10px;
      font-weight: 600;
    }

    .preview-content {
      min-height: 120px;
      padding: 12px;
      overflow: auto;
      border-radius: 8px;
      background: #fff;
      color: #333;
    }

    @media (max-width: 640px) {
      .content-type-selector {
        grid-template-columns: 1fr;
      }
    }
  }

  @keyframes mail-content-saving {
    0%,
    100% {
      opacity: 0.4;
      transform: scale(0.8);
    }

    50% {
      opacity: 1;
      transform: scale(1.2);
    }
  }
</style>
