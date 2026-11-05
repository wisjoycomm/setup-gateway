package main

import "math"

// composeOverlay turns two text renderings into premultiplied BGRA pixels, written
// into colour. colour holds the text on black and mask the same text in white on
// black, so the mask gives each pixel's text coverage. Only the background is see
// through: text stays fully opaque however low bgAlpha is.
func composeOverlay(colour, mask []byte, width, height, radius int, bg [3]byte, bgAlpha byte) {
	for y := range height {
		for x := range width {
			i := (y*width + x) * 4
			rest := 255 - int(mask[i+1])
			alpha := int(bgAlpha)*rest/255 + int(mask[i+1])
			edge := cornerCoverage(x, y, width, height, radius)
			for c := range 3 {
				value := min(int(colour[i+c])+int(bg[c])*int(bgAlpha)/255*rest/255, alpha)
				colour[i+c] = byte(float64(value) * edge)
			}
			colour[i+3] = byte(float64(alpha) * edge)
		}
	}
}

// cornerCoverage is how much of a pixel lies inside the rounded rectangle.
func cornerCoverage(x, y, width, height, radius int) float64 {
	dx := cornerDistance(x, width, radius)
	dy := cornerDistance(y, height, radius)
	if dx == 0 || dy == 0 {
		return 1
	}
	return math.Max(0, math.Min(1, float64(radius)+0.5-math.Hypot(dx, dy)))
}

// cornerDistance is how far a pixel centre sits inside the corner zone on one axis.
func cornerDistance(at, size, radius int) float64 {
	center := float64(at) + 0.5
	return math.Max(0, math.Max(float64(radius)-center, center-float64(size-radius)))
}
