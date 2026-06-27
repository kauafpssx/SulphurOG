package domain

import (
	"fmt"
	"time"
)

type LogFile struct {
	ID             string      `json:"id"`
	MessageID      int         `json:"message_id"`
	FileID         string      `json:"file_id"`
	SourceURL      string      `json:"source_url"`
	GroupURL       string      `json:"group_url"`
	Filename       string      `json:"filename"`
	FileSize       int64       `json:"file_size"`
	SizeFormatted  string      `json:"size_formatted"`
	ContentHash    string      `json:"content_hash"`
	Date           time.Time   `json:"date"`
	Password       string      `json:"password"`
	FileLocation   interface{} `json:"-"`
}

func UnixToDate(timestamp int64) time.Time {
	return time.Unix(timestamp, 0)
}

func FormatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.0fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.0fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
