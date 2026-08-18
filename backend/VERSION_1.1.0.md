# Auto Pro v1.1.0 编译信息

## 版本号
1.1.0

## 编译时间
2024-08-04

## MD5 校验和
- Linux x86_64: `133bdcaea6747b29274dc688899a7a28`
- Windows x86_64: `5ea0f3c6d2c6f6549def4bdb5030f1a9`
- macOS Apple Silicon: `9f98d44c39f33fa55b0e48dcfbc2d7d3`
- macOS Intel: `f4407d02fb3c0c740d4ab11c39fe8e0c`

## 编译产物

### 后端二进制文件
- `auto_pro_linux_amd64_v1.1.0` - Linux x86_64 版本 (11M)
- `auto_pro_windows_amd64_v1.1.0.exe` - Windows x86_64 版本 (11M)
- `auto_pro_darwin_arm64_v1.1.0` - macOS Apple Silicon 版本 (11M)
- `auto_pro_darwin_amd64_v1.1.0` - macOS Intel 版本 (11M)

## 编译参数
- Go 版本: 1.22+
- 编译标志: `-ldflags="-s -w -X 'main.Version=1.1.0'"`
- 静态资源: 使用 go:embed 嵌入 static/* 目录

## 版本更新内容 (v1.1.0)

### 新增功能
1. **代理商为指定用户开通授权**
   - 新增 API: `GET /api/agent-panel/users/options` - 搜索用户选项
   - 代理商购买授权时可选择授权归属用户
   - 支持按邮箱搜索用户（大小写不敏感）

2. **管理员分配配额时搜索代理商**
   - 分配配额弹窗中代理商选择器支持本地搜索
   - 编辑已有配额时代理商字段保持禁用

3. **用户端代理商查询**
   - 新增 API: `GET /api/user-panel/agent-query` - 公开代理商查询接口
   - 用户端首页新增代理商账号查询功能
   - 显示代理商账号、名称和等级信息

4. **翻转卡片查询界面**
   - 将授权查询和代理商查询合并为一个翻转卡片
   - 使用 CSS 3D transform 实现流畅翻转动画
   - 顶部切换按钮快速切换查询类型

### 优化改进
1. **配额逻辑调整**
   - 单价字段可以为 0（仅作记录）
   - 总配额默认值改为 0，范围 0-999999
   - 移除 "-1 表示不限" 的说明
   - 修复 UNSIGNED INT 字段不能存负数的问题

2. **管理员登录跳转修复**
   - 修复管理员登录后错误跳转到 `/user/login` 的问题
   - 增强 redirect 参数过滤逻辑

### 技术改进
- 后端路由注册优化
- 前端组件交互体验提升
- 响应式布局适配移动端

## 环境说明
由于编译环境限制（无外网、无法绑定端口），前端资源未重新构建。
本版本使用后端占位 static 文件，可正常编译运行。
生产环境部署时需要单独构建前端并替换 backend/static 目录内容。

## 部署说明
1. 选择对应平台的二进制文件
2. 确保已配置 `AUTO_PRO_DATA_DIR` 环境变量（可选）
3. 首次运行会自动初始化数据库
4. 默认监听端口: 19127 (可通过 PORT 环境变量修改)