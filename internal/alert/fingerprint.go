package alert

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
)

// GenerateFingerprint 生成告警指纹
func GenerateFingerprint(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	hash := md5.Sum([]byte(strings.Join(parts, ",")))
	return fmt.Sprintf("%x", hash)
}

