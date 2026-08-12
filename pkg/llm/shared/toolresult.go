package shared

import (
	"log"
	"unicode/utf8"

	"github.com/kcaldas/genie/pkg/config"
)

const DefaultMaxToolTextBytes = 128 * 1024
const MinMaxToolTextBytes = 4 * 1024
const DefaultMaxBatchTextBytes = 512 * 1024
const DisabledToolTextCap = -1

func ToolResultLimitsFromEnv(configManager config.Manager) ToolResultLimits {
	return ToolResultLimits{
		MaxTextBytes:      MaxToolTextBytesFromEnv(configManager),
		MaxBatchTextBytes: configManager.GetIntWithDefault("GENIE_MAX_TOOL_BATCH_BYTES", DefaultMaxBatchTextBytes),
	}
}

func MaxToolTextBytesFromEnv(configManager config.Manager) int {
	limit := configManager.GetIntWithDefault("GENIE_MAX_TOOL_RESULT_BYTES", DefaultMaxToolTextBytes)
	switch {
	case limit <= 0:
		return DisabledToolTextCap
	case limit < MinMaxToolTextBytes:
		log.Printf("GENIE_MAX_TOOL_RESULT_BYTES=%d is below the %d-byte floor; using the floor",
			limit, MinMaxToolTextBytes)
		return MinMaxToolTextBytes
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
