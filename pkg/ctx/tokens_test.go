package ctx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokens_EmptyString(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
}

func TestEstimateTokens_ShortString(t *testing.T) {
	// "Hi" = 2 chars → ceil(2/4) = 1
	assert.Equal(t, 1, EstimateTokens("Hi"))
}

func TestEstimateTokens_ExactMultiple(t *testing.T) {
	// 8 chars → 8/4 = 2
	assert.Equal(t, 2, EstimateTokens("12345678"))
}

func TestEstimateTokens_NotExactMultiple(t *testing.T) {
	// 9 chars → ceil(9/4) = 3
	assert.Equal(t, 3, EstimateTokens("123456789"))
}

func TestEstimateTokens_LargeString(t *testing.T) {
	content := strings.Repeat("a", 1000)
	assert.Equal(t, 250, EstimateTokens(content))
}

func TestEstimateTokens_SingleChar(t *testing.T) {
	assert.Equal(t, 1, EstimateTokens("x"))
}
