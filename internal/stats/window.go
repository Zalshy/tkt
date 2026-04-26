package stats

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func ParseWindow(value string) (time.Duration, error) {
	if value == "" {
		return 0, fmt.Errorf("window is empty")
	}
	if strings.HasSuffix(value, "d") {
		daysStr := strings.TrimSuffix(value, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid window %q: use a positive duration like 24h, 7d, or 30d", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid window %q: use a positive duration like 24h, 7d, or 30d", value)
	}
	return d, nil
}
