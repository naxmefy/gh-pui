// Package layout maps the terminal width (in columns, not pixels) to a breakpoint;
// each view decides what to drop as the window gets narrower.
package layout

type Breakpoint int

const (
	XS Breakpoint = iota // < 60: phone portrait
	SM                   // 60–99: split pane, phone landscape
	MD                   // 100–139: tablet, half screen
	LG                   // 140–179: laptop
	XL                   // >= 180: wide monitor
)

func For(width int) Breakpoint {
	switch {
	case width >= 180:
		return XL
	case width >= 140:
		return LG
	case width >= 100:
		return MD
	case width >= 60:
		return SM
	}
	return XS
}

func (b Breakpoint) String() string {
	return [...]string{"xs", "sm", "md", "lg", "xl"}[b]
}
