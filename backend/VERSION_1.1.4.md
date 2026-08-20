# v1.1.4 版本说明

## 更新内容

1. 新增活动管理
2. 新增自助开通代理商
3. 新增密钥搭建站点上限控制
4. 优化在线更新
5. 新增代理商等级自定义
6. 新增 v2 签名算法，兼容 v1 算法，后续强制替换 v2 算法尽早更换

## 产物

| 文件 | 平台 |
|---|---|
| `auto_pro_linux_amd64_v1.1.4` | Linux x86_64 |
| `auto_pro_windows_amd64_v1.1.4.exe` | Windows x86_64 |
| `auto_pro_darwin_amd64_v1.1.4` | macOS Intel |
| `auto_pro_darwin_arm64_v1.1.4` | macOS Apple Silicon |

## 编译命令

```bash
cd backend
GOCACHE="$PWD/../.cache/go-build" go build -ldflags="-s -w -X 'main.Version=1.1.4'" -o auto_pro_linux_amd64_v1.1.4 .
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=1.1.4'" -o auto_pro_windows_amd64_v1.1.4.exe .
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=1.1.4'" -o auto_pro_darwin_amd64_v1.1.4 .
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X 'main.Version=1.1.4'" -o auto_pro_darwin_arm64_v1.1.4 .
```