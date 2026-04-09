package whitelist

import (
	"testing"

	"go.uber.org/zap"
)

func TestWhitelistManager(t *testing.T) {
	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatalf("创建日志器失败: %v", err)
	}

	mgr := NewManager(logger)

	testCases := []struct {
		name      string
		ignoreIPs []string
		testIP    string
		expected  bool
	}{
		{
			name:      "单个IP匹配",
			ignoreIPs: []string{"192.168.1.1"},
			testIP:    "192.168.1.1",
			expected:  true,
		},
		{
			name:      "单个IP不匹配",
			ignoreIPs: []string{"192.168.1.1"},
			testIP:    "192.168.1.2",
			expected:  false,
		},
		{
			name:      "CIDR匹配",
			ignoreIPs: []string{"192.168.1.0/24"},
			testIP:    "192.168.1.100",
			expected:  true,
		},
		{
			name:      "CIDR不匹配",
			ignoreIPs: []string{"192.168.1.0/24"},
			testIP:    "10.0.0.1",
			expected:  false,
		},
		{
			name:      "混合白名单",
			ignoreIPs: []string{"192.168.1.1", "10.0.0.0/8"},
			testIP:    "10.1.1.1",
			expected:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mgr.Load(tc.ignoreIPs)
			result := mgr.Contains(tc.testIP)
			if result != tc.expected {
				t.Errorf("期望 %v, 实际 %v", tc.expected, result)
			}
		})
	}
}
