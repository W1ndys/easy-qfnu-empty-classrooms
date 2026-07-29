package service

import (
	"fmt"
	"strings"
	"time"
)

const legacyStorageTimeFormat = "2006-01-02 15:04:05.999999999"

func nowUTCForStorage() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func utcTimestampForAPI(value string) (string, error) {
	value = strings.TrimSpace(value)
	formats := []string{
		time.RFC3339Nano,
		legacyStorageTimeFormat,
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		parsed, err := time.ParseInLocation(format, value, time.UTC)
		if err == nil {
			return parsed.UTC().Format(time.RFC3339Nano), nil
		}
	}
	return "", fmt.Errorf("不支持的时间格式 %q", value)
}
