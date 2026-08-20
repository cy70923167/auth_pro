<template>
  <div class="developer-doc-page">
    <ElCard shadow="never" class="doc-card">
      <template #header>
        <div class="doc-header">
          <div>
            <h2>开发文档</h2>
            <p>项目授权对接方式、接口参数和推荐接入流程。</p>
          </div>
          <ElTag type="primary" size="large">License API</ElTag>
        </div>
      </template>

      <section class="doc-section">
        <h3>一、对接目标</h3>
        <p>
          业务项目在运行时向授权系统发起校验请求，授权系统根据应用、域名/IP/密钥、授权状态和到期时间返回校验结果。
          项目侧只需要在关键入口增加一次校验，即可控制系统是否允许继续运行。
        </p>
      </section>

      <section class="doc-section">
        <h3>二、准备工作</h3>
        <ElTable :data="prepareRows" border>
          <ElTableColumn prop="item" label="配置项" width="160" />
          <ElTableColumn prop="desc" label="说明" />
        </ElTable>
      </section>

      <section class="doc-section">
        <h3>三、授权校验接口</h3>
        <div class="api-box">
          <div><strong>请求方式：</strong><ElTag>POST</ElTag></div>
          <div><strong>接口地址：</strong><code>/api/license/verify</code></div>
          <div><strong>Content-Type：</strong><code>application/json</code></div>
        </div>

        <h4>请求参数</h4>
        <ElTable :data="requestRows" border>
          <ElTableColumn prop="field" label="字段" width="150" />
          <ElTableColumn prop="required" label="是否必填" width="100" />
          <ElTableColumn prop="desc" label="说明" />
        </ElTable>

        <h4>请求示例</h4>
        <pre><code>{{ requestExample }}</code></pre>
      </section>

      <section class="doc-section">
        <h3>四、响应结构</h3>
        <h4>校验通过</h4>
        <pre><code>{{ successExample }}</code></pre>

        <h4>校验失败</h4>
        <pre><code>{{ failExample }}</code></pre>
      </section>

      <section class="doc-section">
        <h3>五、推荐接入流程</h3>
        <ElSteps direction="vertical" :active="5" finish-status="success">
          <ElStep title="创建应用" description="在后台管理中创建应用，获得 appKey/appSecret。" />
          <ElStep title="创建授权" description="为用户或项目创建域名、泛域名、IP 或密钥授权。" />
          <ElStep title="项目启动校验" description="业务项目启动时请求授权校验接口。" />
          <ElStep
            title="关键操作二次校验"
            description="建议在登录、核心业务入口或定时任务中进行二次校验。"
          />
          <ElStep title="失败熔断" description="校验失败时停止关键功能，并提示授权状态。" />
        </ElSteps>
      </section>

      <section class="doc-section">
        <h3>六、接入伪代码</h3>
        <pre><code>{{ pseudoCode }}</code></pre>
      </section>

      <section class="doc-section">
        <h3>七、状态说明</h3>
        <ElTable :data="statusRows" border>
          <ElTableColumn prop="status" label="状态" width="140" />
          <ElTableColumn prop="desc" label="说明" />
        </ElTable>
      </section>

      <section class="doc-section">
        <h3>八、应用版本检查</h3>
        <div class="api-box">
          <div><strong>请求方式：</strong><ElTag>POST</ElTag></div>
          <div><strong>接口地址：</strong><code>/api/app/version/check</code></div>
          <div><strong>鉴权方式：</strong>有效授权 + 应用密钥 HMAC-SHA256 签名</div>
        </div>

        <h4>请求参数</h4>
        <ElTable :data="versionRequestRows" border>
          <ElTableColumn prop="field" label="字段" width="150" />
          <ElTableColumn prop="required" label="是否必填" width="100" />
          <ElTableColumn prop="desc" label="说明" />
        </ElTable>

        <h4>请求与响应示例</h4>
        <pre><code>{{ versionCheckExample }}</code></pre>

        <h4>客户端升级顺序</h4>
        <pre><code>{{ versionUpdateFlow }}</code></pre>

        <ElAlert
          type="warning"
          :closable="false"
          title="下载地址是短期令牌地址。客户端应及时下载并校验文件大小与 MD5，不要持久化该地址。"
        />
      </section>
    </ElCard>
  </div>
</template>

<script setup lang="ts">
  defineOptions({ name: 'DeveloperDoc' })

  const prepareRows = [
    { item: 'appKey', desc: '应用唯一标识，在应用管理中获取。' },
    { item: 'appSecret', desc: '应用密钥，用于签名或服务端可信校验。' },
    { item: '授权目标', desc: '域名、泛域名、IP 或 licenseKey。' },
    { item: '服务地址', desc: '授权系统后端访问地址，例如 https://license.example.com。' }
  ]

  const requestRows = [
    { field: 'appKey', required: '是', desc: '应用标识。' },
    { field: 'domain', required: '否', desc: '当前访问域名，域名/IP 授权使用。' },
    { field: 'serverIp', required: '否', desc: '当前服务器 IP，IP 授权或风控追踪使用。' },
    { field: 'licenseKey', required: '否', desc: '密钥授权时传入。' },
    { field: 'timestamp', required: '是', desc: 'Unix 秒级时间戳，服务端允许前后 10 分钟偏差。' },
    {
      field: 'signVersion',
      required: '是',
      desc: '签名版本，新接入必须传 "v2"；v1 仅兼容历史绑定站点。'
    },
    {
      field: 'sign',
      required: '是',
      desc: 'v2: HMAC-SHA256(appSecret, "v2\\nappKey\\nlicenseKey\\ndomain\\nserverIp\\ntimestamp")；v1: MD5(appKey+target+timestamp+appSecret)。'
    }
  ]

  const statusRows = [
    { status: 'pass', desc: '授权有效，可以继续运行。' },
    { status: 'fail', desc: '授权不存在、目标不匹配或签名错误。' },
    { status: 'expired', desc: '授权已过期，需要续费或重新开通。' },
    { status: 'blacklisted', desc: '目标命中黑名单，应立即阻断访问。' },
    {
      status: 'signature_upgrade_required',
      desc: '密钥新站点首次绑定必须使用 v2 签名，当前 SDK 需升级。'
    },
    {
      status: 'site_limit_exceeded',
      desc: '密钥已绑定站点数达到上限，拒绝新站点，不吊销已绑定站点。'
    }
  ]

  const versionRequestRows = [
    { field: 'appKey', required: '是', desc: '应用管理中生成的应用标识。' },
    { field: 'currentVersion', required: '是', desc: '客户端当前版本号，例如 1.2.0。' },
    {
      field: 'domain/serverIp/licenseKey',
      required: '三选一',
      desc: '与有效授权匹配的授权目标，密钥授权时同时上报 domain/serverIp 以定位站点。'
    },
    { field: 'timestamp', required: '是', desc: 'Unix 秒级时间戳，服务端允许前后 10 分钟偏差。' },
    { field: 'signVersion', required: '是', desc: '签名版本，新接入必须传 "v2"。' },
    {
      field: 'sign',
      required: '是',
      desc: 'v2: HMAC-SHA256(appSecret, "v2\\nappKey\\ncurrentVersion\\nlicenseKey\\ndomain\\nserverIp\\ntimestamp")；v1: HMAC-SHA256(appSecret, appKey\\ncurrentVersion\\n目标\\ntimestamp)。'
    }
  ]

  const requestExample = `POST /api/license/verify
Content-Type: application/json

{
  "appKey": "cms_pro",
  "domain": "www.example.com",
  "serverIp": "203.0.113.10",
  "licenseKey": "",
  "timestamp": 1760000000,
  "signVersion": "v2",
  "sign": "hmac_sha256_hex"
}

// v2 规范串（固定 6 段，字段为空时保留空串）：
// v2\\ncms_pro\\n\\nwww.example.com\\n203.0.113.10\\n1760000000
// sign = HMAC-SHA256(appSecret, 上述规范串)`

  const successExample = `{
  "code": 200,
  "msg": "授权有效",
  "data": {
    "result": "pass",
    "appName": "CMS Pro",
    "expireAt": "2027-12-31 23:59:59"
  }
}`

  const failExample = `{
  "code": 403,
  "msg": "授权无效或已过期",
  "data": {
    "result": "fail",
    "reason": "license_not_found"
  }
}`

  const pseudoCode = `启动项目
  读取 appKey/appSecret/licenseKey
  获取当前 domain/serverIp
  生成 timestamp
  按 "v2\\nappKey\\nlicenseKey\\ndomain\\nserverIp\\ntimestamp" 生成 HMAC-SHA256 v2 签名
  请求 /api/license/verify
  如果 result == pass:
    允许系统继续运行
  否则:
    关闭核心功能并展示授权异常提示`

  const versionCheckExample = `POST /api/app/version/check
Content-Type: application/json

{
  "appKey": "cms_pro",
  "currentVersion": "1.2.0",
  "licenseKey": "license_xxx",
  "domain": "www.example.com",
  "serverIp": "203.0.113.10",
  "timestamp": 1784788800,
  "signVersion": "v2",
  "sign": "hmac_sha256_hex"
}

// v2 规范串（固定 7 段，字段为空时保留空串）：
// v2\\ncms_pro\\n1.2.0\\nlicense_xxx\\nwww.example.com\\n203.0.113.10\\n1784788800
// sign = HMAC-SHA256(appSecret, 上述规范串)

{
  "code": 200,
  "msg": "",
  "data": {
    "hasUpdate": true,
    "currentVersion": "1.2.0",
    "latestVersion": "1.5.0",
    "version": "1.5.0",
    "title": "性能优化与问题修复",
    "changelog": "1. 优化启动速度\\n2. 修复已知问题",
    "downloadUrl": "/api/app/version/download?token=短期签名令牌",
    "fileSizeMb": 18.625,
    "fileMd5": "d41d8cd98f00b204e9800998ecf8427e",
    "forceUpdate": true,
    "minVersion": "1.1.0",
    "publishedAt": "2026-07-23 12:00:00",
    "updates": [
      {
        "version": "1.3.0",
        "title": "数据库升级",
        "changelog": "增加索引",
        "updateSql": "ALTER TABLE example ADD INDEX idx_enabled (enabled);"
      },
      {
        "version": "1.5.0",
        "title": "性能优化与问题修复",
        "changelog": "1. 优化启动速度\\n2. 修复已知问题",
        "updateSql": ""
      }
    ]
  }
}`

  const versionUpdateFlow = `检查更新:
  读取 appKey/appSecret、当前版本和授权目标
  获取当前 domain/serverIp
  生成 timestamp
  按 "v2\\nappKey\\ncurrentVersion\\nlicenseKey\\ndomain\\nserverIp\\ntimestamp" 生成 HMAC-SHA256 v2 签名
  POST /api/app/version/check
  如果 hasUpdate == false:
    结束更新
  如果 forceUpdate == true:
    禁止用户跳过本次更新
  按 updates 的版本升序执行每一步 updateSql
  下载 latestVersion 更新包并校验文件大小和 MD5
  安装成功后，将本地版本写为 latestVersion
  任一步失败时回滚并保留原版本`
</script>

<style scoped lang="scss">
  .developer-doc-page {
    padding: 16px;
  }

  .doc-card {
    max-width: 1100px;
  }

  .doc-header {
    display: flex;
    gap: 16px;
    align-items: center;
    justify-content: space-between;

    h2 {
      margin: 0 0 8px;
      font-size: 22px;
      font-weight: 700;
    }

    p {
      margin: 0;
      color: var(--el-text-color-secondary);
    }
  }

  .doc-section {
    margin-bottom: 28px;

    h3 {
      margin: 0 0 14px;
      font-size: 17px;
      font-weight: 700;
    }

    h4 {
      margin: 18px 0 10px;
      font-size: 14px;
      font-weight: 700;
    }

    p {
      line-height: 1.8;
      color: var(--el-text-color-regular);
    }
  }

  .api-box {
    display: grid;
    gap: 10px;
    padding: 14px 16px;
    background: var(--el-fill-color-lighter);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 10px;
  }

  code {
    padding: 2px 6px;
    font-family: 'Roboto Mono', monospace;
    background: var(--el-fill-color);
    border-radius: 4px;
  }

  pre {
    padding: 16px;
    overflow: auto;
    line-height: 1.7;
    color: #d1d5db;
    background: #111827;
    border-radius: 10px;

    code {
      padding: 0;
      color: inherit;
      background: transparent;
    }
  }
</style>
