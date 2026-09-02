package main

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrustedProxyCIDRsDefaults(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")

	got := trustedProxyCIDRs()
	want := []string{"172.22.0.0/16", "172.25.0.0/16"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("默认可信代理网段错误: got=%v want=%v", got, want)
	}
}

func TestTrustedProxyCIDRsTrimsConfiguredValues(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", " 10.0.0.0/8, 192.0.2.0/24 ,, ")

	got := trustedProxyCIDRs()
	want := []string{"10.0.0.0/8", "192.0.2.0/24"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("配置的可信代理网段解析错误: got=%v want=%v", got, want)
	}
}

func TestTrustedProxyCIDRsPreserveClientIPAcrossProxyChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.ForwardedByClientIP = true
	router.RemoteIPHeaders = []string{"X-Forwarded-For", "X-Real-IP"}
	if err := router.SetTrustedProxies([]string{"172.22.0.0/16", "172.25.0.0/16"}); err != nil {
		t.Fatalf("设置可信代理失败: %v", err)
	}
	router.GET("/", func(c *gin.Context) {
		c.String(200, c.ClientIP())
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.25.0.4:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.7, 172.22.0.3")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got, want := resp.Body.String(), "198.51.100.7"; got != want {
		t.Fatalf("未正确提取真实访客 IP: got=%q want=%q", got, want)
	}
}

func TestUntrustedPeerCannotOverrideClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.ForwardedByClientIP = true
	router.RemoteIPHeaders = []string{"X-Forwarded-For", "X-Real-IP"}
	if err := router.SetTrustedProxies([]string{"172.22.0.0/16", "172.25.0.0/16"}); err != nil {
		t.Fatalf("设置可信代理失败: %v", err)
	}
	router.GET("/", func(c *gin.Context) {
		c.String(200, c.ClientIP())
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.9:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.8")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got, want := resp.Body.String(), "198.51.100.9"; got != want {
		t.Fatalf("不可信对端可以伪造访客 IP: got=%q want=%q", got, want)
	}
}
