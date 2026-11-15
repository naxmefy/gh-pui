package layout

import "testing"

func TestFor(t *testing.T) {
	for width, want := range map[int]Breakpoint{
		0: XS, 59: XS, 60: SM, 99: SM, 100: MD, 139: MD, 140: LG, 179: LG, 180: XL, 400: XL,
	} {
		if got := For(width); got != want {
			t.Errorf("For(%d) = %s, want %s", width, got, want)
		}
	}
}
