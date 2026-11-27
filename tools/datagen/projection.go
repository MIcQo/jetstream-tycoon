package main

import (
	"math"
	"strconv"
	"strings"
)

// naturalEarth1 projects (lon, lat) in degrees with the Natural Earth I projection
// (Šavrič, Jenny, Patterson & Jenny 2011; same coefficients as d3.geoNaturalEarth1).
// The result is in radians-scale units: x ∈ about [-2.73, 2.73], y ∈ about [-1.42, 1.42].
func naturalEarth1(lon, lat float64) (x, y float64) {
	l := lon * math.Pi / 180
	p := lat * math.Pi / 180
	p2 := p * p
	p4 := p2 * p2
	x = l * (0.8707 - 0.131979*p2 + p4*(-0.013791+p4*(0.003971*p2-0.001529*p4)))
	y = p * (1.007226 + p2*(0.015085+p4*(-0.044475+0.028874*p2-0.005916*p4)))
	return x, y
}

// MapFrame converts projected coordinates into SVG viewBox units with y pointing down.
type MapFrame struct {
	Scale  float64
	Width  float64
	Height float64
}

// frame for a map Width units wide. The height follows from the projection's extent.
func newFrame(width float64) MapFrame {
	xMax, _ := naturalEarth1(180, 0)
	_, yMax := naturalEarth1(0, 90)
	scale := width / (2 * xMax)
	return MapFrame{Scale: scale, Width: width, Height: math.Round(2 * yMax * scale)}
}

// toView projects (lon, lat) into the frame.
func (f MapFrame) toView(lon, lat float64) (float64, float64) {
	x, y := naturalEarth1(lon, lat)
	return f.Width/2 + x*f.Scale, f.Height/2 - y*f.Scale
}

// svgPath encodes polygons as a compact SVG path: absolute "M" per ring, then
// relative "l" segments rounded to one decimal, dropping points that round onto
// the previous one, and "z" to close. Use fill-rule="evenodd" so holes render.
func (f MapFrame) svgPath(polygons [][][][2]float64) string {
	var b strings.Builder
	for _, poly := range polygons {
		for _, ring := range poly {
			writeRing(&b, f, ring)
		}
	}
	return b.String()
}

func writeRing(b *strings.Builder, f MapFrame, ring [][2]float64) {
	// Work in integer tenths so relative steps add up exactly.
	pts := make([][2]int64, 0, len(ring))
	for _, p := range ring {
		x, y := f.toView(p[0], p[1])
		q := [2]int64{int64(math.Round(x * 10)), int64(math.Round(y * 10))}
		if n := len(pts); n > 0 && pts[n-1] == q {
			continue
		}
		pts = append(pts, q)
	}
	if len(pts) > 1 && pts[0] == pts[len(pts)-1] {
		pts = pts[:len(pts)-1] // "z" closes the ring
	}
	if len(pts) < 3 {
		return // collapsed to a point or line at this precision
	}
	b.WriteString("M")
	b.WriteString(tenths(pts[0][0]) + "," + tenths(pts[0][1]))
	b.WriteString("l")
	for i := 1; i < len(pts); i++ {
		if i > 1 {
			b.WriteString(" ")
		}
		b.WriteString(tenths(pts[i][0]-pts[i-1][0]) + "," + tenths(pts[i][1]-pts[i-1][1]))
	}
	b.WriteString("z")
}

// tenths formats an integer number of tenths with at most one decimal ("12.3", "-4", "0.5").
func tenths(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatInt(v/10, 10)
	if r := v % 10; r != 0 {
		s += "." + strconv.FormatInt(r, 10)
	}
	if neg {
		s = "-" + s
	}
	return s
}
