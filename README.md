# auth_pro

`auth_pro` 是一个授权管理与反盗版后台系统，包含后台管理端、代理端、用户端和授权校验 API。项目采用前后端分离开发，生产环境可将前端产物内嵌到 Go 后端统一部署。

## 核心功能

- 授权管理：应用、应用版本、套餐、授权码、授权状态、授权校验日志管理。
- 用户体系：管理员登录、用户注册登录、用户资料、余额和授权购买。
- 代理体系：代理登录、代理余额、授权购买、代理等级和额度管理。
- 反盗版：盗版追踪、告警、黑名单和数据报表。
- 系统管理：菜单、角色、用户、系统配置、邮件配置和邮件日志。
- 安装向导：首次运行时配置数据库并创建管理员账号。

## 技术栈

### 后端

- Go 1.22
- Gin
- MySQL
- JWT

### 前端

- Vue 3
- TypeScript
- Vite
- Pinia
- Vue Router
- Element Plus
- Tailwind CSS

## 目录结构

```text
auth_pro/
├── backend/              # Go 后端服务
│   ├── config/           # 配置读取与数据库配置持久化
│   ├── handler/          # API 处理器与业务入口
│   ├── middleware/       # CORS、JWT 等中间件
│   ├── model/            # 数据模型
│   ├── service/          # 服务层
│   ├── static/           # 前端构建产物，供后端内嵌部署
│   └── main.go           # 后端入口
├── database/             # 数据库结构文件
├── frontend/             # Vue 前端项目
│   ├── src/api/          # API 请求封装
│   ├── src/router/       # 路由与权限处理
│   ├── src/store/        # Pinia 状态管理
│   └── src/views/        # 页面模块
└── scripts/              # 运维脚本
```

## 环境要求

- Go >= 1.22
- Node.js >= 20.19.0
- pnpm >= 8.8.0
- MySQL 5.7+ 或 MySQL 8.x

## 本地开发

### 1. 启动后端

```bash
cd backend
go mod download
go run .
```

默认端口为 `19127`，可通过环境变量修改：

```bash
PORT=19127 go run .
```

数据库配置由安装向导写入 `backend/db.json`，安装完成后会生成 `backend/install.lock`。

### 2. 启动前端

```bash
cd frontend
pnpm install
pnpm dev
```

开发环境下，前端通过 Vite 代理将 `/api` 请求转发到 `http://localhost:19127`。

### 3. 首次安装

启动前后端后，在浏览器访问前端地址，进入安装流程：

1. 填写 MySQL 连接信息。
2. 初始化数据库表结构。
3. 创建管理员账号。
4. 进入后台管理系统。

## 生产构建

### 1. 构建前端

```bash
cd frontend
pnpm install
pnpm build
```

### 2. 同步前端产物到后端

```bash
rm -rf ../backend/static/*
cp -R dist/* ../backend/static/
```

### 3. 构建并运行后端

```bash
cd ../backend
go build -o auth_pro .
PORT=19127 ./auth_pro
```

也可以使用项目提供的脚本重启后端：

```bash
./scripts/restart-backend.sh
```

## 重要配置

| 配置项               | 说明                                             | 默认值                   |
| -------------------- | ------------------------------------------------ | ------------------------ |
| `PORT`               | 后端服务端口                                     | `19127`                  |
| `AUTO_PRO_DATA_DIR`  | 后端运行数据目录，用于保存配置、更新包和运行数据 | 当前运行目录             |
| `VITE_API_PROXY_URL` | 前端开发代理目标地址                             | `http://localhost:19127` |

## API 入口

- `/api/install/*`：安装流程接口。
- `/api/auth/login`：后台管理员登录。
- `/api/license/verify`：公开授权校验接口。
- `/api/app/version/check`：应用客户端使用有效授权和 HMAC-SHA256 签名检查版本。
- `/api/app/version/download?token=...`：使用版本检查或管理员接口签发的短期令牌下载本地更新包。
- `/api/agent-panel/*`：代理端接口。
- `/api/user-panel/*`：用户端接口。
- `/api/*`：后台管理接口，除公开接口外默认需要 JWT 鉴权。

## 部署说明

生产环境推荐同源部署：前端构建后放入 `backend/static`，由 Go 后端统一提供静态资源和 `/api` 接口。这样可以减少跨域配置，并保持授权校验、管理后台和前端页面的一致部署入口。
