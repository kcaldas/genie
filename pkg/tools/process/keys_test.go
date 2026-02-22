package process

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeKeys_Enter(t *testing.T) {
	b, err := EncodeKeys([]string{"Enter"})
	require.NoError(t, err)
	assert.Equal(t, []byte{'\r'}, b)
}

func TestEncodeKeys_CtrlC(t *testing.T) {
	b, err := EncodeKeys([]string{"C-c"})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x03}, b)
}

func TestEncodeKeys_CtrlD(t *testing.T) {
	b, err := EncodeKeys([]string{"C-d"})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x04}, b)
}

func TestEncodeKeys_CtrlZ(t *testing.T) {
	b, err := EncodeKeys([]string{"C-z"})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x1a}, b)
}

func TestEncodeKeys_CtrlUpperCase(t *testing.T) {
	b, err := EncodeKeys([]string{"C-C"})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x03}, b)
}

func TestEncodeKeys_Arrows(t *testing.T) {
	tests := []struct {
		key      string
		expected []byte
	}{
		{"Up", []byte{0x1b, '[', 'A'}},
		{"Down", []byte{0x1b, '[', 'B'}},
		{"Right", []byte{0x1b, '[', 'C'}},
		{"Left", []byte{0x1b, '[', 'D'}},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			b, err := EncodeKeys([]string{tt.key})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, b)
		})
	}
}

func TestEncodeKeys_SpecialKeys(t *testing.T) {
	tests := []struct {
		key      string
		expected []byte
	}{
		{"Escape", []byte{0x1b}},
		{"Tab", []byte{'\t'}},
		{"Backspace", []byte{0x7f}},
		{"Space", []byte{' '}},
		{"Delete", []byte{0x1b, '[', '3', '~'}},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			b, err := EncodeKeys([]string{tt.key})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, b)
		})
	}
}

func TestEncodeKeys_FunctionKeys(t *testing.T) {
	b, err := EncodeKeys([]string{"F1"})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x1b, 'O', 'P'}, b)

	b, err = EncodeKeys([]string{"F12"})
	require.NoError(t, err)
	assert.Equal(t, []byte{0x1b, '[', '2', '4', '~'}, b)
}

func TestEncodeKeys_LiteralChar(t *testing.T) {
	b, err := EncodeKeys([]string{"a"})
	require.NoError(t, err)
	assert.Equal(t, []byte{'a'}, b)
}

func TestEncodeKeys_MultipleKeys(t *testing.T) {
	b, err := EncodeKeys([]string{"h", "e", "l", "l", "o", "Enter"})
	require.NoError(t, err)
	assert.Equal(t, []byte{'h', 'e', 'l', 'l', 'o', '\r'}, b)
}

func TestEncodeKeys_UnknownKey(t *testing.T) {
	_, err := EncodeKeys([]string{"FooBar"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown key")
}

func TestEncodeKeys_Empty(t *testing.T) {
	b, err := EncodeKeys([]string{})
	require.NoError(t, err)
	assert.Nil(t, b)
}

func TestEncodeKeys_InvalidCtrl(t *testing.T) {
	_, err := EncodeKeys([]string{"C-1"})
	assert.Error(t, err)
}

func TestEncodeKeys_AllCtrlLetters(t *testing.T) {
	for ch := byte('a'); ch <= 'z'; ch++ {
		key := "C-" + string(ch)
		b, err := EncodeKeys([]string{key})
		require.NoError(t, err, "key: %s", key)
		assert.Equal(t, []byte{ch - 'a' + 1}, b, "key: %s", key)
	}
}
