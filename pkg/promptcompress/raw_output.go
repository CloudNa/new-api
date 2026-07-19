package promptcompress

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var rtkRawOutputFailurePattern = regexp.MustCompile(`(?i)\b(error|failed|failure|exception|traceback|panic|fatal|critical|TS\d{4}|FAIL)\b`)

var rtkRawOutputSecretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}\b`), "[redacted_secret]"},
	{regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{16,}\b`), "[redacted_secret]"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[redacted_secret]"},
	{regexp.MustCompile(`(?i)((?:api[_-]?key|token|secret|password)\s*[:=]\s*)("[^"]+"|'[^']+'|[^\s]+)`), "${1}[redacted_secret]"},
	{regexp.MustCompile(`(?i)(Authorization:\s*Bearer\s+)[A-Za-z0-9._~+/-]+=*`), "${1}[redacted_secret]"},
}

func maybePersistRtkRawOutput(raw string, config Config, command string) (*RtkRawOutputPointer, error) {
	retention := NormalizeRtkRawOutputRetention(config.RawOutputRetention)
	if retention == RtkRawOutputRetentionNever || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	failure := isLikelyRtkFailureOutput(raw)
	if retention == RtkRawOutputRetentionFailures && !failure {
		return nil, nil
	}
	maxBytes := normalizedRtkRawOutputMaxBytes(config.RawOutputMaxBytes)
	redactedText, redacted := redactRtkRawOutput(safeUtf8Slice(raw, maxBytes))
	now := time.Now().UnixMilli()
	id := safeRtkRawOutputID(fmt.Sprintf("%d:%s:%d:%s", now, command, len(raw), redactedText))
	dir := filepath.Join(rtkRawOutputDataDir(), "rtk", "raw-output")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	filePath := filepath.Join(dir, fmt.Sprintf("%d-tool-output-%s.log", now, id))
	if err := os.WriteFile(filePath, []byte(redactedText), 0o600); err != nil {
		return nil, err
	}
	return &RtkRawOutputPointer{
		ID:       id,
		Path:     filePath,
		Bytes:    len([]byte(redactedText)),
		SHA256:   hex.EncodeToString(common.Sha256Raw([]byte(redactedText))),
		Redacted: redacted,
	}, nil
}

func redactRtkRawOutput(value string) (string, bool) {
	text := value
	redacted := false
	for _, item := range rtkRawOutputSecretPatterns {
		next := item.pattern.ReplaceAllString(text, item.replacement)
		if next != text {
			redacted = true
			text = next
		}
	}
	next, count := redactSecrets(text)
	if count > 0 {
		redacted = true
		text = next
	}
	return text, redacted
}

func isLikelyRtkFailureOutput(value string) bool {
	return rtkRawOutputFailurePattern.MatchString(value)
}

func safeUtf8Slice(value string, maxBytes int) string {
	maxBytes = normalizedRtkRawOutputMaxBytes(maxBytes)
	if len([]byte(value)) <= maxBytes {
		return value
	}
	var builder strings.Builder
	bytes := 0
	for _, char := range value {
		part := string(char)
		partBytes := len([]byte(part))
		if bytes+partBytes > maxBytes {
			break
		}
		builder.WriteString(part)
		bytes += partBytes
	}
	return fmt.Sprintf("%s\n\n--- truncated at %d bytes ---", builder.String(), maxBytes)
}

func normalizedRtkRawOutputMaxBytes(value int) int {
	if value <= 0 {
		return 1048576
	}
	if value < 1024 {
		return 1024
	}
	return value
}

func safeRtkRawOutputID(seed string) string {
	hash := hex.EncodeToString(common.Sha256Raw([]byte(seed)))
	if len(hash) > 24 {
		return hash[:24]
	}
	return hash
}

func rtkRawOutputDataDir() string {
	if dir := strings.TrimSpace(os.Getenv("DATA_DIR")); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "new-api-rtk")
}
