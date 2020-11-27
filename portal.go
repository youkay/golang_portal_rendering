// Portal Rendering
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

var W float32 = 608
var H float32 = 480

var EyeHeight float32 = 6
var DuckHeight float32 = 2.5
var HeadMargin float32 = 1
var KneeHeight float32 = 2
var hFov float32 = 0.73 * H
var vFov float32 = 0.2 * H

type sector struct {
	floor, ceil float32
	vertex      []sdl.FPoint
	neighbors   []int32
	npoints     uint32
}

var sectors []sector = nil
var NumSectors int32 = 0

type player struct {
	where, velocity                struct{ x, y, z float32 }
	angle, anglesin, anglecos, yaw float32
	sector                         int32
}

var pl player

func min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float32) float32 {
	if a < b {
		return b
	}
	return a
}

// 値をmi~maの範囲で制限する
func clamp(a, mi, ma float32) float32 {
	return min(max(a, mi), ma)
}

// 外積
func vxs(x0, y0, x1, y1 float32) float32 {
	return x0*y1 - x1*y0
}

// Overlap 二つの範囲が重なっているかを返す
func Overlap(a0, a1, b0, b1 float32) bool {
	return min(a0, a1) <= max(b0, b1) && min(b0, b1) <= max(a0, a1)
}

// IntersectBox 四角形が重なっているかを返す
func IntersectBox(x0, y0, x1, y1, x2, y2, x3, y3 float32) bool {
	return Overlap(x0, x1, x2, x3) && Overlap(y0, y1, y2, y3)
}

// PointSide px,pyの点がx0,y0,x1,y1の線のどちら側にいるかを返す
func PointSide(px, py, x0, y0, x1, y1 float32) float32 {
	return vxs(x1-x0, x1-y0, px-x0, py-y0)
}

// Intersect 二つのベクトルの接点を返す
// http://marupeke296.com/COL_2D_No10_SegmentAndSegment.html
func Intersect(x1, y1, x2, y2, x3, y3, x4, y4 float32) (x, y float32) {
	x = vxs(vxs(x1, y1, x2, y2), x1-x2, vxs(x3, y3, x4, y4), x3-x4) / vxs(x1-x2, y1-y2, x3-x4, y3-y4)
	y = vxs(vxs(x1, y1, x2, y2), y1-y2, vxs(x3, y3, x4, y4), y3-y4) / vxs(x1-x2, y1-y2, x3-x4, y3-y4)

	return
}

// LoadData マップを読み込む
func LoadData() {
	fp, err := os.Open("map-clear.txt")
	if err != nil {
		panic(err)
	}
	defer fp.Close()

	var vert []sdl.FPoint

	reader := bufio.NewReaderSize(fp, 256)

	for {
		b, _, err := reader.ReadLine()
		buf := string(b)

		if err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			panic(err)
		}

		if len(buf) > 0 {
			fmt.Printf("buf[0]: %s\n", string(buf[0]))
			switch buf[0] {
			case 'v': // vertex
				vlines := strings.Split(string(buf[len("vertex")+1:]), "\t")
				xbuf, err := strconv.ParseFloat(vlines[0], 32)
				if err != nil {
					panic(err)
				}
				x := float32(xbuf)

				for i, vline := range vlines {
					if i == 0 {
						continue
					}
					ylist := strings.Split(vline, " ")
					for _, y := range ylist {
						ybuf, err := strconv.ParseFloat(y, 32)
						if err != nil {
							panic(err)
						}
						vert = append(vert, sdl.FPoint{X: x, Y: float32(ybuf)})
					}
				}
			case 's': // sector
				sline := strings.Split(string(buf[len("sector")+1:]), "\t")

				// sectorのフロア/床の高さを取得
				heights := strings.Split(sline[0], " ")
				floorHeight, err := strconv.ParseFloat(heights[0], 32)
				if err != nil {
					panic(err)
				}
				ceilHeight, err := strconv.ParseFloat(heights[1], 32)
				if err != nil {
					panic(err)
				}

				// sectorで利用する頂点/隣接セクターを取得
				vstrings := strings.Split(sline[1], " ")
				vnums := []int32{}
				for _, v := range vstrings {
					if v == "" {
						continue
					}
					vnum, err := strconv.Atoi(v)
					if err != nil {
						panic(err)
					}
					vnums = append(vnums, int32(vnum))
				}

				vtxNum := len(vnums) / 2
				vtx := []sdl.FPoint{}
				for i := 0; i < vtxNum; i++ {
					vtx = append(vtx, vert[vnums[i]])
				}

				// sectorの頂点はループさせておく
				vtx = append([]sdl.FPoint{vert[vnums[vtxNum-1]]}, vtx...)

				fmt.Printf("vtx: %v\n", vtx)

				// sectorの各辺がどのセクターと隣接しているかを取得する
				neighbors := []int32{}
				for i := 0; i < vtxNum; i++ {
					neighbor := vnums[i+vtxNum]
					if err != nil {
						panic(err)
					}
					neighbors = append(neighbors, int32(neighbor))
				}
				fmt.Printf("neighbors: %v\n", neighbors)

				sectors = append(sectors, sector{floor: float32(floorHeight), ceil: float32(ceilHeight), vertex: vtx, npoints: uint32(len(vtx)), neighbors: neighbors})
			case 'p': // player
				fmt.Sscanf(string(buf[len("player")+1:]), "%f %f %f	%d", &pl.where.x, &pl.where.y, &pl.angle, &pl.sector)
			}

		}
	}

	fmt.Printf("vert: %v\n", vert)
	fmt.Printf("player: %v\n", pl)
}

func main() {
	LoadData()
}
