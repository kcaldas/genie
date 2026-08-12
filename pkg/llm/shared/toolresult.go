package shared

import (
	"log"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kcaldas/genie/pkg/config"
)

const DefaultMaxToolTextBytes = 20 * 1024 * 1024
const MinMaxToolTextBytes = 4 * 1024
const DefaultMaxBatchTextBytes = -1
const DefaultMaxToolBlobBytes = 20 * 1024 * 1024
const DisabledToolTextCap = -1

func ToolResultLimitsFromEnv(configManager config.Manager) ToolResultLimits {
	return ToolResultLimits{
		MaxTextBytes:      MaxToolTextBytesFromEnv(configManager),
		MaxBatchTextBytes: maxToolBytesFromEnv(configManager, "GENIE_MAX_TOOL_BATCH_BYTES", DefaultMaxBatchTextBytes, MinMaxToolTextBytes),
		MaxBlobBytes:      MaxToolBlobBytesFromEnv(configManager),
	}
}

func MaxToolTextBytesFromEnv(configManager config.Manager) int {
	return maxToolBytesFromEnv(configManager, "GENIE_MAX_TOOL_RESULT_BYTES", DefaultMaxToolTextBytes, MinMaxToolTextBytes)
}

func MaxToolBlobBytesFromEnv(configManager config.Manager) int {
	return maxToolBytesFromEnv(configManager, "GENIE_MAX_ATTACHMENT_BYTES", DefaultMaxToolBlobBytes, 0)
}

func maxToolBytesFromEnv(configManager config.Manager, key string, defaultValue, floor int) int {
	raw := strings.TrimSpace(configManager.GetStringWithDefault(key, ""))
	if raw == "" {
		return defaultValue
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("%s=%q is not a valid byte limit; using %d", key, raw, defaultValue)
		return defaultValue
	}
	switch {
	case limit <= 0:
		return DisabledToolTextCap
	case floor > 0 && limit < floor:
		log.Printf("%s=%d is below the %d-byte floor; using the floor", key, limit, floor)
		return floor
	default:
		return limit
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncateUTF8(text string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(text) <= max {
		return text
	}
	cut := text[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}
