// Package layout computes where each panel of the TUI goes for a given
// terminal size. It is pure arithmetic so the arrangement can be tested
// without rendering anything.
package layout

// Fixed row heights, in cells, including borders.
const (
	InputHeight  = 3
	StatusHeight = 1

	// StatusRightWidth is the fixed width of the right status segment.
	StatusRightWidth = 36
)

// RightMode says which panel, if any, occupies the right column.
type RightMode int

const (
	RightNone RightMode = iota
	RightDebug
	RightText
	RightDiff
)

// State is the part of the UI state that affects geometry.
type State struct {
	Right  RightMode
	Zoomed bool // the right panel takes most of the width
}

// Rect is a screen rectangle in cells.
type Rect struct{ X, Y, W, H int }

// Empty reports whether the rectangle has no area.
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Contains reports whether the cell (x, y) lies inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Rects is the computed arrangement. Right is empty when no right panel
// is shown.
type Rects struct {
	Messages     Rect
	Right        Rect
	Input        Rect
	StatusLeft   Rect
	StatusCenter Rect
	StatusRight  Rect
}

// Compute lays out a w by h terminal. The centre area is split between
// the messages panel and the right panel by weight (2:1, or 1:4 when
// zoomed); the input row is three cells high; the status row is one cell
// split left 2 : centre 1 with a fixed-width right segment.
func Compute(w, h int, s State) Rects {
	if w <= 0 || h <= 0 {
		return Rects{}
	}
	var r Rects

	centreH := max(h-InputHeight-StatusHeight, 0)
	r.Messages = Rect{X: 0, Y: 0, W: w, H: centreH}
	if s.Right != RightNone {
		msgW, rightW := 2, 1
		if s.Zoomed {
			msgW, rightW = 1, 4
		}
		rw := w * rightW / (msgW + rightW)
		r.Messages.W = w - rw
		r.Right = Rect{X: w - rw, Y: 0, W: rw, H: centreH}
	}

	inputY := centreH
	inputH := min(InputHeight, max(h-centreH-StatusHeight, 0))
	r.Input = Rect{X: 0, Y: inputY, W: w, H: inputH}

	statusY := inputY + inputH
	if statusY < h {
		right := min(StatusRightWidth, w)
		rem := w - right
		left := rem * 2 / 3
		centre := rem - left
		r.StatusLeft = Rect{X: 0, Y: statusY, W: left, H: 1}
		r.StatusCenter = Rect{X: left, Y: statusY, W: centre, H: 1}
		r.StatusRight = Rect{X: left + centre, Y: statusY, W: right, H: 1}
	}
	return r
}
