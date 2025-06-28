package layout

type ScreenMode int

const (
	SCREEN_NORMAL ScreenMode = iota
	SCREEN_HALF
	SCREEN_FULL
)

type ScreenManager struct {
	mode          ScreenMode
	focusedWindow string
	width         int
	height        int
}

func NewScreenManager() *ScreenManager {
	return &ScreenManager{
		mode: SCREEN_NORMAL,
	}
}

func (sm *ScreenManager) SetMode(mode ScreenMode) {
	sm.mode = mode
}

func (sm *ScreenManager) GetMode() ScreenMode {
	return sm.mode
}

func (sm *ScreenManager) SetFocusedWindow(windowName string) {
	sm.focusedWindow = windowName
}

func (sm *ScreenManager) GetFocusedWindow() string {
	return sm.focusedWindow
}

func (sm *ScreenManager) SetDimensions(width, height int) {
	sm.width = width
	sm.height = height
}

func (sm *ScreenManager) GetDimensions() (int, int) {
	return sm.width, sm.height
}

func (sm *ScreenManager) ToggleMode() {
	switch sm.mode {
	case SCREEN_NORMAL:
		sm.mode = SCREEN_HALF
	case SCREEN_HALF:
		sm.mode = SCREEN_FULL
	case SCREEN_FULL:
		sm.mode = SCREEN_NORMAL
	}
}

func (sm *ScreenManager) IsPortraitMode() bool {
	return sm.width <= 84 && sm.height > 45
}

func (sm *ScreenManager) IsCompactMode() bool {
	return sm.height < 24 || sm.width < 60
}

func (sm *ScreenManager) IsNarrowMode() bool {
	return sm.width < 100
}

func (sm *ScreenManager) ShouldHideSidePanels() bool {
	switch sm.mode {
	case SCREEN_HALF, SCREEN_FULL:
		return sm.focusedWindow == "main" || sm.focusedWindow == "response"
	default:
		return false
	}
}

func (sm *ScreenManager) GetMainPanelWeight() int {
	switch sm.mode {
	case SCREEN_NORMAL:
		return 3
	case SCREEN_HALF:
		if sm.ShouldHideSidePanels() {
			return 5
		}
		return 4
	case SCREEN_FULL:
		return 5
	default:
		return 3
	}
}

func (sm *ScreenManager) GetSidePanelWeight() int {
	if sm.ShouldHideSidePanels() {
		return 0
	}
	
	switch sm.mode {
	case SCREEN_NORMAL:
		return 2
	case SCREEN_HALF:
		return 1
	case SCREEN_FULL:
		return 0
	default:
		return 2
	}
}