package helpers

type Helpers struct {
	Clipboard *Clipboard
	Config    *ConfigManager
}

func NewHelpers() (*Helpers, error) {
	configHelper, err := NewConfigManager()
	if err != nil {
		return nil, err
	}

	return &Helpers{
		Clipboard: NewClipboard(),
		Config:    configHelper,
	}, nil
}

