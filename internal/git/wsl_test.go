package git

import (
	"reflect"
	"testing"
)

func TestSafeDirEnv(t *testing.T) {
	dirs := []string{"//a/*", "//b/*"}
	tests := []struct {
		count string
		want  map[string]string
	}{
		{"", map[string]string{
			"GIT_CONFIG_COUNT":   "2",
			"GIT_CONFIG_KEY_0":   "safe.directory",
			"GIT_CONFIG_VALUE_0": "//a/*",
			"GIT_CONFIG_KEY_1":   "safe.directory",
			"GIT_CONFIG_VALUE_1": "//b/*",
		}},
		{"3", map[string]string{
			"GIT_CONFIG_COUNT":   "5",
			"GIT_CONFIG_KEY_3":   "safe.directory",
			"GIT_CONFIG_VALUE_3": "//a/*",
			"GIT_CONFIG_KEY_4":   "safe.directory",
			"GIT_CONFIG_VALUE_4": "//b/*",
		}},
		{"junk", map[string]string{
			"GIT_CONFIG_COUNT":   "2",
			"GIT_CONFIG_KEY_0":   "safe.directory",
			"GIT_CONFIG_VALUE_0": "//a/*",
			"GIT_CONFIG_KEY_1":   "safe.directory",
			"GIT_CONFIG_VALUE_1": "//b/*",
		}},
	}
	for _, tt := range tests {
		if got := safeDirEnv(tt.count, dirs); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("safeDirEnv(%q) = %v, want %v", tt.count, got, tt.want)
		}
	}
}
