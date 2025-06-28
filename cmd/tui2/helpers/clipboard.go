package helpers

import (
	"github.com/atotto/clipboard"
)

type ClipboardHelper struct{}

func NewClipboardHelper() *ClipboardHelper {
	return &ClipboardHelper{}
}

func (h *ClipboardHelper) Copy(text string) error {
	return clipboard.WriteAll(text)
}

func (h *ClipboardHelper) Paste() (string, error) {
	return clipboard.ReadAll()
}

func (h *ClipboardHelper) IsAvailable() bool {
	err := clipboard.WriteAll("test")
	if err != nil {
		return false
	}
	
	_, err = clipboard.ReadAll()
	return err == nil
}