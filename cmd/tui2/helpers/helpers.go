package helpers

type Helpers struct {
	Clipboard    *ClipboardHelper
	Config       *ConfigHelper
}

func NewHelpers() (*Helpers, error) {
	configHelper, err := NewConfigHelper()
	if err != nil {
		return nil, err
	}
	
	return &Helpers{
		Clipboard:    NewClipboardHelper(),
		Config:       configHelper,
	}, nil
}