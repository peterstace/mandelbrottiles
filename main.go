package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
)

func main() {
	listenAddr := flag.String("listen-addr", ":8080", "address to listen for tile requests on")
	flag.Parse()
	log.Fatal(http.ListenAndServe(*listenAddr, tileServer()))
}

const tileSize = 256

func tileServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		coords, err := extractTileCoords(r.URL.Path)
		if err != nil {
			log.Println(err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		tile := renderTile(coords)
		if err := png.Encode(w, tile); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error: " + err.Error()))
			return
		}
	})
}

type TileCoords struct {
	Z, X, Y int
}

var pathRegex = regexp.MustCompile(`^/(\d+)/(\d+)/(\d+)\.png$`)

func extractTileCoords(path string) (TileCoords, error) {
	matches := pathRegex.FindStringSubmatch(path)
	if len(matches) != 4 {
		return TileCoords{}, fmt.Errorf("not enough matches, got %d", len(matches))
	}

	var coords TileCoords
	var err error
	coords.Z, err = strconv.Atoi(matches[1])
	if err != nil {
		return TileCoords{}, fmt.Errorf("extracting z: %v", err)
	}
	coords.X, err = strconv.Atoi(matches[2])
	if err != nil {
		return TileCoords{}, fmt.Errorf("extracting x: %v", err)
	}
	coords.Y, err = strconv.Atoi(matches[3])
	if err != nil {
		return TileCoords{}, fmt.Errorf("extracting y: %v", err)
	}

	max := 1 << coords.Z
	if coords.X < 0 || coords.X >= max || coords.Y < 0 || coords.Y >= max {
		return TileCoords{}, fmt.Errorf("invalid tile coordinates: %v", coords)
	}

	return coords, nil
}

func renderTile(coords TileCoords) image.Image {
	extent := tileExtent(coords)
	tile := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	for i := 0; i < tileSize; i++ {
		for j := 0; j < tileSize; j++ {
			c := Vector{
				extent.Min.X + (extent.Max.X-extent.Min.X)*float64(i)/tileSize,
				extent.Min.Y + (extent.Max.Y-extent.Min.Y)*float64(j)/tileSize,
			}
			iterationCount := mandelbrot(c)
			colour := escapeColour(iterationCount)
			tile.SetRGBA(i, j, colour)
		}
	}
	return tile
}

func escapeColour(iterationCount float64) color.RGBA {
	iterationCount *= 25 // artistically chosen multiplier
	return hslToRGB(math.Mod(iterationCount+360, 360), 0.5, 0.5)
}

func hslToRGB(hue, saturation, lightness float64) color.RGBA {
	if hue < 0 || hue > 360 {
		panic("hue must be from 0 to 360")
	}
	if saturation < 0 || saturation > 1 {
		panic("saturation must be between 0 and 1")
	}
	if lightness < 0 || lightness > 1 {
		panic("lightness must be between 0 and 1")
	}

	c := (1 - math.Abs(2*lightness-1)) * saturation // chroma
	hueAdj := hue / 60
	x := c * (1 - math.Abs(math.Mod(hueAdj, 2)-1))

	var r, g, b float64
	switch {
	case hueAdj <= 1:
		r, g, b = c, x, 0
	case hueAdj <= 2:
		r, g, b = x, c, 0
	case hueAdj <= 3:
		r, g, b = 0, c, x
	case hueAdj <= 4:
		r, g, b = 0, x, c
	case hueAdj <= 5:
		r, g, b = x, 0, c
	case hueAdj <= 6:
		r, g, b = c, 0, x
	default:
		panic(false)
	}

	m := lightness - 0.5*c
	r += m
	g += m
	b += m

	if r < 0 || r > 1.0 {
		panic(r)
	}
	if g < 0 || g > 1.0 {
		panic(g)
	}
	if b < 0 || b > 1.0 {
		panic(b)
	}

	return color.RGBA{uint8(r * 0xff), uint8(g * 0xff), uint8(b * 0xff), 0xff}
}

// mandelbrot returns 0 for numbers in the mandelbrot set, or the smoothed
// iteration count before escape has been confirmed.
func mandelbrot(c Vector) float64 {
	const maxIter = 1000
	var z Vector
	iterate := func() {
		z = Vector{z.X*z.X - z.Y*z.Y + c.X, 2*z.X*z.Y + c.Y}
	}
	for i := 0; i < maxIter; i++ {
		iterate()
		if z.X*z.X+z.Y*z.Y > 4 {
			iterate()
			iterate()
			modulus := math.Sqrt(z.X*z.X + z.Y*z.Y)
			return float64(i) - math.Log(math.Log(modulus))/math.Log(2)
		}
	}
	return 0
}

type Vector struct {
	X, Y float64
}

func (v Vector) Sub(u Vector) Vector {
	return Vector{v.X - u.X, v.Y - u.Y}
}

func (v Vector) Scale(f float64) Vector {
	return Vector{v.X * f, v.Y * f}
}

type Extent struct {
	Min, Max Vector
}

func tileExtent(coords TileCoords) Extent {
	extent := Extent{
		Min: Vector{
			float64(coords.X) / float64(uint(1)<<uint(coords.Z)),
			float64(coords.Y) / float64(uint(1)<<uint(coords.Z)),
		},
		Max: Vector{
			float64(coords.X+1) / float64(uint(1)<<uint(coords.Z)),
			float64(coords.Y+1) / float64(uint(1)<<uint(coords.Z)),
		},
	}

	extent.Min = extent.Min.Sub(Vector{0.5, 0.5})
	extent.Max = extent.Max.Sub(Vector{0.5, 0.5})
	extent.Min = extent.Min.Scale(4)
	extent.Max = extent.Max.Scale(4)
	return extent
}
