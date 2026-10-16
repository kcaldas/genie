package layout

import "testing"

func TestComputeDefaultArrangement(t *testing.T) {
	r := Compute(120, 40, State{})
	want := Rects{
		Messages:     Rect{0, 0, 120, 36},
		Input:        Rect{0, 36, 120, 3},
		StatusLeft:   Rect{0, 39, 56, 1},
		StatusCenter: Rect{56, 39, 28, 1},
		StatusRight:  Rect{84, 39, 36, 1},
	}
	if r != want {
		t.Fatalf("got %+v\nwant %+v", r, want)
	}
}

func TestComputeRightPanelWeights(t *testing.T) {
	r := Compute(120, 40, State{Right: RightDebug})
	if r.Messages.W != 80 || r.Right != (Rect{80, 0, 40, 36}) {
		t.Fatalf("2:1 split wrong: messages=%+v right=%+v", r.Messages, r.Right)
	}
	z := Compute(120, 40, State{Right: RightDiff, Zoomed: true})
	if z.Messages.W != 24 || z.Right != (Rect{24, 0, 96, 36}) {
		t.Fatalf("1:4 zoomed split wrong: messages=%+v right=%+v", z.Messages, z.Right)
	}
	if z.Messages.W+z.Right.W != 120 {
		t.Fatal("columns must tile the width exactly")
	}
}

func TestComputeZoomWithoutRightPanelIsIgnored(t *testing.T) {
	r := Compute(100, 30, State{Zoomed: true})
	if !r.Right.Empty() || r.Messages.W != 100 {
		t.Fatalf("zoom without a right panel changed layout: %+v", r)
	}
}

func TestComputeTinyTerminals(t *testing.T) {
	if r := Compute(0, 10, State{}); r != (Rects{}) {
		t.Fatalf("zero width must yield empty rects: %+v", r)
	}
	r := Compute(20, 3, State{})
	if r.Messages.H != 0 || r.Input.H != 2 || r.StatusLeft.Y != 2 {
		t.Fatalf("3-row terminal: %+v", r)
	}
	r = Compute(20, 1, State{})
	if r.Input.H != 0 || r.StatusLeft.H != 1 {
		t.Fatalf("1-row terminal keeps the status row: %+v", r)
	}
	r = Compute(10, 20, State{})
	if r.StatusRight.W != 10 || r.StatusLeft.W != 0 {
		t.Fatalf("narrow terminal gives the fixed segment what fits: %+v", r)
	}
}

func TestRectContains(t *testing.T) {
	r := Rect{X: 5, Y: 5, W: 10, H: 2}
	for _, c := range []struct {
		x, y int
		in   bool
	}{{5, 5, true}, {14, 6, true}, {15, 6, false}, {4, 5, false}, {5, 7, false}} {
		if got := r.Contains(c.x, c.y); got != c.in {
			t.Errorf("Contains(%d,%d) = %v, want %v", c.x, c.y, got, c.in)
		}
	}
}

func TestComputeGrowsTheInputWithItsRows(t *testing.T) {
	r := Compute(100, 40, State{InputRows: 4})
	if r.Input.H != 6 || r.Input.Y != 33 || r.Messages.H != 33 {
		t.Fatalf("4-row prompt: %+v", r)
	}
	r = Compute(100, 12, State{InputRows: 20})
	if r.Input.H != 6 || r.Messages.H != 5 {
		t.Fatalf("prompt must leave the chat %d rows: %+v", minChatRows, r)
	}
	r = Compute(100, 6, State{InputRows: 20})
	if r.Input.H != 3 || r.Messages.H != 2 {
		t.Fatalf("tiny terminal keeps the single-row prompt: %+v", r)
	}
}
