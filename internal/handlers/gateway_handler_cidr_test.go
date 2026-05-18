package httphandlers

import (
	"net"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestCheckCIDR(t *testing.T) {
	t.Parallel()

	_, ipNet1, _ := net.ParseCIDR("10.0.0.0/8")
	_, ipNet2, _ := net.ParseCIDR("192.168.1.0/24")
	_, ipNet3, _ := net.ParseCIDR("172.16.0.0/12")

	tests := []struct {
		name           string
		remoteAddr     string
		blockedIPNets  []*net.IPNet
		allowedIPNets  []*net.IPNet
		expectedResult bool
	}{
		{
			name:           "no restrictions should allow all",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  nil,
			allowedIPNets:  nil,
			expectedResult: true,
		},
		{
			name:           "empty CIDR lists should allow all",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  []*net.IPNet{},
			allowedIPNets:  []*net.IPNet{},
			expectedResult: true,
		},
		{
			name:           "invalid IP should be denied (fail closed)",
			remoteAddr:     "invalid-ip:12345",
			blockedIPNets:  nil,
			allowedIPNets:  nil,
			expectedResult: false,
		},
		{
			name:           "blocked CIDR contains IP should deny",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  []*net.IPNet{ipNet1}, // 10.0.0.0/8 contains 10.0.0.1
			allowedIPNets:  nil,
			expectedResult: false,
		},
		{
			name:           "blocked CIDR does not contain IP should allow",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  []*net.IPNet{ipNet2}, // 192.168.1.0/24 does not contain 10.0.0.1
			allowedIPNets:  nil,
			expectedResult: true,
		},
		{
			name:           "allowed CIDR contains IP should allow",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  nil,
			allowedIPNets:  []*net.IPNet{ipNet1}, // 10.0.0.0/8 contains 10.0.0.1
			expectedResult: true,
		},
		{
			name:           "allowed CIDR does not contain IP should deny",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  nil,
			allowedIPNets:  []*net.IPNet{ipNet2}, // 192.168.1.0/24 does not contain 10.0.0.1
			expectedResult: false,
		},
		{
			name:           "blocked takes precedence over allowed",
			remoteAddr:     "10.0.0.1:12345",
			blockedIPNets:  []*net.IPNet{ipNet1}, // 10.0.0.0/8 contains 10.0.0.1
			allowedIPNets:  []*net.IPNet{ipNet1}, // same CIDR in allowlist
			expectedResult: false,
		},
		{
			name:           "multiple allowed CIDRs matching one should allow",
			remoteAddr:     "192.168.1.100:12345",
			blockedIPNets:  nil,
			allowedIPNets:  []*net.IPNet{ipNet1, ipNet2, ipNet3}, // 10.0.0.0/8, 192.168.1.0/24, 172.16.0.0/12
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gin.SetMode(gin.TestMode)
			handler := &GatewayHandler{logger: nil}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("GET", "/", nil)
			if tt.remoteAddr != "" {
				c.Request.RemoteAddr = tt.remoteAddr
			}

			route := &domain.GatewayRoute{
				BlockedIPNets: tt.blockedIPNets,
				AllowedIPNets: tt.allowedIPNets,
			}

			result := handler.checkCIDR(c, route)
			assert.Equal(t, tt.expectedResult, result, tt.name)
		})
	}
}
