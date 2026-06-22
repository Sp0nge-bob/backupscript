package interval

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const MinDuration = time.Minute

var pattern = regexp.MustCompile(`^(\d+)([smhdw])$`)

func Parse(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("интервал не указан")
	}

	m := pattern.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("неверный формат %q: используйте 30m, 6h, 7d, 1w", s)
	}

	var n int
	if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("число должно быть больше 0")
	}

	var d time.Duration
	switch m[2] {
	case "s":
		d = time.Duration(n) * time.Second
	case "m":
		d = time.Duration(n) * time.Minute
	case "h":
		d = time.Duration(n) * time.Hour
	case "d":
		d = time.Duration(n) * 24 * time.Hour
	case "w":
		d = time.Duration(n) * 7 * 24 * time.Hour
	default:
		return 0, fmt.Errorf("неизвестная единица в %q", s)
	}

	if d < MinDuration {
		return 0, fmt.Errorf("минимальный интервал — 1m")
	}

	return d, nil
}

func Describe(s string) string {
	d, err := Parse(s)
	if err != nil {
		return s
	}
	return Humanize(d)
}

func Humanize(d time.Duration) string {
	if d%(7*24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return d.String()
}