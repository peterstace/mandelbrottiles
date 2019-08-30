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

	// TODO: validate tile

	return coords, nil
}

func renderTile(coords TileCoords) image.Image {
	log.Printf("rendering: %v", coords)

	extent := tileExtent(coords)
	log.Printf("extent: %v", extent)

	tile := image.NewGray(image.Rect(0, 0, tileSize, tileSize))
	for i := 0; i < tileSize; i++ {
		for j := 0; j < tileSize; j++ {
			c := Vector{
				extent.Min.X + (extent.Max.X-extent.Min.X)*float64(i)/tileSize,
				extent.Min.Y + (extent.Max.Y-extent.Min.Y)*float64(j)/tileSize,
			}
			colour := color.Gray{0xff}
			if mandelbox(c) {
				colour = color.Gray{}
			}
			tile.SetGray(i, j, colour)
		}
	}
	return tile
}

func mandelbox(c Vector) bool {
	var z Vector
	for i := 0; i < 10; i++ {
		if z.X > 1 {
			z.X = 2 - z.X
		} else if z.X < -1 {
			z.X = -2 - z.X
		}
		if z.Y > 1 {
			z.Y = 2 - z.Y
		} else if z.Y < -1 {
			z.Y = -2 - z.Y
		}
		mag := math.Sqrt(z.X*z.X + z.Y*z.Y)
		if mag < 0.5 {
			z = z.Scale(4)
		} else if mag < 1 {
			z = z.Scale(mag * mag)
		}

		const s = 2
		z = z.Scale(s)
		z = z.Add(c)

		if z.X*z.X+z.Y*z.Y > 100 {
			return false
		}
	}
	return true
}

type Vector struct {
	X, Y float64
}

func (v Vector) Sub(u Vector) Vector {
	return Vector{v.X - u.X, v.Y - u.Y}
}

func (v Vector) Add(u Vector) Vector {
	return Vector{v.X + u.X, v.Y + u.Y}
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
	extent.Min = extent.Min.Scale(16)
	extent.Max = extent.Max.Scale(16)
	return extent
}
