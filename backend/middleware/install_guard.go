package middleware

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"auto_pro/config"

	"github.com/gin-gonic/gin"
)

// InstallGuard 保护安装写接口：
//   - 已安装（install.lock 存在）时拒绝一切安装操作，封死远程重放清库与创建管理员的未鉴权后门；
//   - 未安装时拒绝跨域请求，防止任意网页借浏览器跨域触发安装/抢注。
//
// 无 Origin 的请求（curl 等非浏览器客户端）在未安装状态下放行，保持手动安装流程可用。
func InstallGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.IsInstalled() {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "系统已安装，安装接口已关闭"})
			c.Abort()
			return
		}

		if origin := c.GetHeader("Origin"); origin != "" && !sameOrigin(origin, c.Request.Host) {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "拒绝非本站发起的安装请求"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// sameOrigin 比较浏览器 Origin 与请求 Host 是否同源（按主机名比较，忽略端口差异）。
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	originHost := strings.ToLower(u.Hostname())
	requestHost := hostnameOnly(host)
	return originHost != "" && originHost == requestHost
}

func hostnameOnly(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(host)
}