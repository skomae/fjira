package app

func MinInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func MaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func ClampInt(v, min, max int) int {
	if v > max {
		return max
	}
	if v < min {
		return min
	}
	return v
}

// ScrollPageSize returns how many rows a PgUp/PgDn should advance a scrollable
// view, given the screen height. Visible content height is the screen height
// minus chrome (top/bottom bars + a margin), capped at half the screen so a
// page-jump never feels disorienting, and floored so very small terminals still
// scroll noticeably. Shared by the issue detail view and the text editor so
// paging feels the same everywhere.
func ScrollPageSize(screenY int) int {
	const chromeRows = 16 // top/bottom bars (12) + margin (4)
	const minPage = 5
	visibleHeight := screenY - chromeRows
	pageSize := ClampInt(visibleHeight, 1, screenY/2)
	if pageSize < minPage {
		pageSize = minPage
	}
	return pageSize
}
