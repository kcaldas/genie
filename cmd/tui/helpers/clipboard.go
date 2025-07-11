package helpers

import (
	"github.com/atotto/clipboard"
)

type Clipboard struct{}

func NewClipboard() *Clipboard {
	return &Clipboard{}
}

func (h *Clipboard) Copy(text string) error {
	return clipboard.WriteAll(text)
}

func (h *Clipboard) Paste() (string, error) {
	return clipboard.ReadAll()
}

func (h *Clipboard) IsAvailable() bool {
	err := clipboard.WriteAll("test")
	if err != nil {
		return false
	}

	_, err = clipboard.ReadAll()
	return err == nil
}

