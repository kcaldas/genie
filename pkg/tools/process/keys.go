package process

import "fmt"

// EncodeKeys converts tmux-style key names to raw bytes.
// Supported keys: Enter, C-a through C-z, Escape, Tab, Backspace, Space,
// Up, Down, Right, Left, Home, End, PgUp, PgDn, Delete, F1-F12.
// Single characters pass through as literals. Unknown multi-char keys return error.
func EncodeKeys(keys []string) ([]byte, error) {
	var result []byte
	for _, key := range keys {
		b, err := encodeKey(key)
		if err != nil {
			return nil, err
		}
		result = append(result, b...)
	}
	return result, nil
}

func encodeKey(key string) ([]byte, error) {
	// Check named keys first
	if b, ok := namedKeys[key]; ok {
		return b, nil
	}

	// Check Ctrl+letter: C-a through C-z
	if len(key) == 3 && key[0] == 'C' && key[1] == '-' {
		ch := key[2]
		if ch >= 'a' && ch <= 'z' {
			return []byte{ch - 'a' + 1}, nil
		}
		if ch >= 'A' && ch <= 'Z' {
			return []byte{ch - 'A' + 1}, nil
		}
		return nil, fmt.Errorf("unknown control key: %q", key)
	}

	// Single character passes through as literal
	if len(key) == 1 {
		return []byte(key), nil
	}

	return nil, fmt.Errorf("unknown key: %q", key)
}

var namedKeys = map[string][]byte{
	"Enter":     {'\r'},
	"Escape":    {'\x1b'},
	"Tab":       {'\t'},
	"Backspace": {'\x7f'},
	"Space":     {' '},

	// Arrow keys (xterm)
	"Up":    {'\x1b', '[', 'A'},
	"Down":  {'\x1b', '[', 'B'},
	"Right": {'\x1b', '[', 'C'},
	"Left":  {'\x1b', '[', 'D'},

	// Navigation
	"Home":   {'\x1b', '[', 'H'},
	"End":    {'\x1b', '[', 'F'},
	"PgUp":   {'\x1b', '[', '5', '~'},
	"PgDn":   {'\x1b', '[', '6', '~'},
	"Delete": {'\x1b', '[', '3', '~'},
	"Insert": {'\x1b', '[', '2', '~'},

	// Function keys (xterm)
	"F1":  {'\x1b', 'O', 'P'},
	"F2":  {'\x1b', 'O', 'Q'},
	"F3":  {'\x1b', 'O', 'R'},
	"F4":  {'\x1b', 'O', 'S'},
	"F5":  {'\x1b', '[', '1', '5', '~'},
	"F6":  {'\x1b', '[', '1', '7', '~'},
	"F7":  {'\x1b', '[', '1', '8', '~'},
	"F8":  {'\x1b', '[', '1', '9', '~'},
	"F9":  {'\x1b', '[', '2', '0', '~'},
	"F10": {'\x1b', '[', '2', '1', '~'},
	"F11": {'\x1b', '[', '2', '3', '~'},
	"F12": {'\x1b', '[', '2', '4', '~'},
}
