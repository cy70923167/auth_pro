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
```

## GitHub Actions 自动发布

工作流文件位于 `.github/workflows/release.yml`，仅负责准备构建环境、调用上述脚本、创建 GitHub Release，并将在线更新清单部署到 GitHub Pages。包结构、版本注入、哈希和清单仍以 `scripts/build-release.sh` 为准。

首次使用前，在 GitHub 仓库的 `Settings -> Pages` 中将 `Source` 设置为 `GitHub Actions`。

正式发布使用语义版本标签：

```bash
git tag v1.2.3
git push origin v1.2.3
```

工作流会发布：

- GitHub Release：`auth_pro-full-v1.2.3.tar.gz`、SHA-256 校验文件及更新清单。
- GitHub Pages：`latest.json` 与 `releases.json`。
- 更新包下载地址：当前标签对应的 GitHub Release 资产地址。

标签必须使用 `v<主版本>.<次版本>.<修订版本>` 格式，也支持 `v1.2.3-rc.1` 预发布标签。Actions 页面中的手动运行入口只用于重新发布已经存在的标签。

本地或其他发布环境可用以下变量覆盖发布地址：

- `AUTO_PRO_UPDATE_PACKAGE_BASE_URL`：更新包下载基址。
- `AUTO_PRO_UPDATE_RELEASES_URL`：历史版本清单地址。
- `AUTO_PRO_UPDATE_MANIFEST_URL`：编译进后端的默认 `latest.json` 地址；部署时仍可用 `AUTO_PRO_UPDATE_URL` 覆盖。
