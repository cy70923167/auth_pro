# v1.1.3 版本说明

## 核心修复

**用户端支付宝支付回跳错误**
- 用户端购买授权订单号前缀由 `LP` 改为 `UP`，与代理端 `LP` 订单隔离
- 支付完成回跳兜底逻辑按订单号前缀区分：`UP` → `/user/purchase`，`LP` → `/agent/purchase`
- 修复用户端支付后误跳代理端后台，导致"加载余额失败"、"加载可开通应用失败"的问题

## 产物

| 文件 | 平台 |
|---|---|
| `auto_pro_linux_amd64_v1.1.3` | Linux x86_64 |
| `auto_pro_windows_amd64_v1.1.3.exe` | Windows x86_64 |
| `auto_pro_darwin_amd64_v1.1.3` | macOS Intel |
| `auto_pro_darwin_arm64_v1.1.3` | macOS Apple Silicon |

## 编译命令

```bash
cd backend
GOCACHE="$PWD/../.cache/go-build" go build -ldflags="-s -w -X 'main.Version=1.1.3'" -o auto_pro_linux_amd64_v1.1.3 .
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=1.1.3'" -o auto_pro_windows_amd64_v1.1.3.exe .
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=1.1.3'" -o auto_pro_darwin_amd64_v1.1.3 .
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X 'main.Version=1.1.3'" -o auto_pro_darwin_arm64_v1.1.3 .
```