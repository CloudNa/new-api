package model

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

const (
	redactedBase64LogValue    = "[redacted base64 payload]"
	redactedDataURLLogValue   = "[redacted data URL]"
	redactedMediaURLLogValue  = "[redacted media URL]"
	redactedLocalPathLogValue = "[redacted local path]"
)

var (
	logDataURLPattern      = regexp.MustCompile(`data:[^"'\s<>]+;base64,[A-Za-z0-9+/=_-]+`)
	logWindowsPathPattern  = regexp.MustCompile(`[A-Za-z]:\\[^\s"'<>]+`)
	logUnixUserPathPattern = regexp.MustCompile(`/(?:Users|home|var|tmp)/[^\s"'<>]+`)
)

func sanitizeLogText(text string) string {
	if text == "" {
		return text
	}
	text = logDataURLPattern.ReplaceAllString(text, redactedDataURLLogValue)
	text = logWindowsPathPattern.ReplaceAllString(text, redactedLocalPathLogValue)
	text = logUnixUserPathPattern.ReplaceAllString(text, redactedLocalPathLogValue)
	if looksLikeLargeBase64(text) {
		return redactedBase64LogValue
	}
	return text
}

func sanitizeLogOther(other map[string]interface{}) map[string]interface{} {
	if other == nil {
		return nil
	}
	sanitized := make(map[string]interface{}, len(other))
	for key, value := range other {
		sanitized[key] = sanitizeLogValue(key, value)
	}
	return sanitized
}

func sanitizeLogValue(key string, value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return sanitizeLogStringValue(key, typed)
	case map[string]interface{}:
		return sanitizeLogOther(typed)
	case []interface{}:
		items := make([]interface{}, len(typed))
		for i, item := range typed {
			items[i] = sanitizeLogValue(key, item)
		}
		return items
	case []string:
		items := make([]string, len(typed))
		for i, item := range typed {
			if sanitized, ok := sanitizeLogValue(key, item).(string); ok {
				items[i] = sanitized
			}
		}
		return items
	case []map[string]interface{}:
		items := make([]map[string]interface{}, len(typed))
		for i, item := range typed {
			items[i] = sanitizeLogOther(item)
		}
		return items
	default:
		return value
	}
}

func sanitizeLogStringValue(key string, value string) string {
	if value == "" {
		return value
	}
	if strings.HasPrefix(strings.TrimSpace(value), "data:") {
		return redactedDataURLLogValue
	}
	if isSensitiveMediaLogKey(key) && isLikelyMediaURL(value) {
		return redactedMediaURLLogValue
	}
	if isSensitivePayloadLogKey(key) && looksLikeLargeBase64(value) {
		return redactedBase64LogValue
	}
	return sanitizeLogText(value)
}

func isSensitivePayloadLogKey(key string) bool {
	key = strings.ToLower(key)
	sensitiveParts := []string{
		"base64",
		"b64",
		"bytes",
		"body",
		"payload",
		"inline_data",
		"inlinedata",
		"response",
		"request",
	}
	for _, part := range sensitiveParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func isSensitiveMediaLogKey(key string) bool {
	key = strings.ToLower(key)
	if key == "request_path" || key == "path" {
		return false
	}
	sensitiveParts := []string{
		"image",
		"video",
		"audio",
		"media",
		"file",
		"download",
		"result_url",
		"source_url",
		"target_url",
		"url",
		"uri",
	}
	for _, part := range sensitiveParts {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func isLikelyMediaURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme == "file" {
		return true
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	path := strings.ToLower(parsed.Path)
	mediaSuffixes := []string{
		".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".svg",
		".mp4", ".mov", ".webm", ".mkv", ".avi", ".mpeg", ".mpg",
		".mp3", ".wav", ".m4a", ".aac", ".ogg", ".flac",
		".pdf", ".zip",
	}
	for _, suffix := range mediaSuffixes {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return strings.Contains(path, "/uploads/") ||
		strings.Contains(path, "/files/") ||
		strings.Contains(path, "/media/") ||
		strings.Contains(path, "/storage/")
}

func looksLikeLargeBase64(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 256 {
		return false
	}
	base64Chars := 0
	for _, r := range value {
		if unicode.IsSpace(r) {
			continue
		}
		if (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '+' || r == '/' || r == '=' || r == '-' || r == '_' {
			base64Chars++
			continue
		}
		return false
	}
	return base64Chars >= 256
}
