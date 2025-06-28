package helpers

type Helpers struct {
	Clipboard    *ClipboardHelper
	Config       *ConfigHelper
	Notification *NotificationHelper
}

func NewHelpers() (*Helpers, error) {
	configHelper, err := NewConfigHelper()
	if err != nil {
		return nil, err
	}
	
	return &Helpers{
		Clipboard:    NewClipboardHelper(),
		Config:       configHelper,
		Notification: NewNotificationHelper(),
	}, nil
}