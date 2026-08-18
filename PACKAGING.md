# 宝塔发布包目录规范

本项目每次编译打包都必须生成“宝塔当前网站根目录直接解压即用”的目录结构。

## 标准目录

```text
auth_pro-full-v1.0.0.tar.gz
├── index.html
├── version.json
├── favicon.ico
├── assets/
│   ├── index-xxxx.js
│   ├── index-xxxx.css
│   └── ...
├── backend/
│   └── auth_pro
└── manifest.json
```

## 必须遵守

- `index.html` 必须位于压缩包根目录。
- `assets/` 必须位于压缩包根目录，并且和 `index.html` 同级。
- 不得在宝塔网站根目录外再嵌套一层 `frontend/`。
- Go 二进制固定放在 `backend/auth_pro`。
- `manifest.json` 中的 `frontendDir` 固定为 `.`。
- `manifest.json` 中的 `backendFile` 固定为 `backend/auth_pro`。

## 解压后的服务器目录

如果宝塔当前网站根目录是 `/www/wwwroot/example.com`，解压后必须是：

```text
/www/wwwroot/example.com/
├── index.html
├── version.json
├── favicon.ico
├── assets/
├── backend/
│   └── auth_pro
└── manifest.json
```

这样浏览器请求 `/assets/index-xxxx.js` 时会命中真实文件，不会 fallback 到 `index.html`。

## 构建命令

macOS / Linux：

```bash
./scripts/build-release.sh 1.0.0
```

Windows PowerShell：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build-release.ps1 -Version 1.0.0
```

输出文件固定为：

```text
release/packages/auth_pro-full-v<版本号>.tar.gz
release/packages/latest.json
release/packages/releases.json
```

构建脚本只生成 `Linux amd64` 后端，版本参数必须匹配 `X.Y.Z`。`latest.json` 中记录平台、文件名、GitHub Release 下载地址、文件大小和 SHA256；`releases.json` 合并保留已有历史版本，并将上一版本标签到当前版本之间的 Git 提交标题自动记录到对应版本的 `notes`。首次发布会记录当前 Git 历史；无 Git 历史时才使用兜底说明。可通过 `AUTO_PRO_RELEASE_NOTES` 显式覆盖本次更新内容（JSON 字符串数组或按行分隔文本）。

## GitHub 自动发布

公开仓库 `cy70923167/auth_pro` 的 Release 工作流由 `vX.Y.Z` tag 触发：

```bash
git tag v1.2.3
git push origin v1.2.3
```

每个 GitHub Release 必须包含：

```text
auth_pro-full-v1.2.3.tar.gz
latest.json
releases.json
```

在线更新默认读取：

```text
https://github.com/cy70923167/auth_pro/releases/latest/download/latest.json
```

服务端可通过 `AUTO_PRO_UPDATE_URL` 指向自建 HTTPS 镜像清单。GitHub 默认源只信任指定仓库的 Release 路径及 GitHub 官方 Release Asset 重定向目标。

## 完整性边界

当前更新链路校验压缩包大小和 SHA256，不校验离线数字签名。SHA256 可以发现下载损坏，但仓库、Actions 或 Release 发布权限一旦被攻破，攻击者仍可同时替换更新包和校验值。建议在仓库设置中启用 Immutable Releases，并严格控制仓库管理员和 Actions 权限。
