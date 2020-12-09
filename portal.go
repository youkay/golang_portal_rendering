// Portal Rendering
package main

import (
	"bufio"
	"fmt"
	"image/color"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

var W int32 = 1024
var H int32 = 680

var EyeHeight float32 = 6
var DuckHeight float32 = 2.5
var HeadMargin float32 = 1
var KneeHeight float32 = 2
var hFov float32 = 0.73 * float32(H)
var vFov float32 = 0.2 * float32(H)

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

func minInt32(a, b int32) int32 {
	return int32(min(float32(a), float32(b)))
}

func max(a, b float32) float32 {
	if a < b {
		return b
	}
	return a
}

func maxInt32(a, b int32) int32 {
	return int32(max(float32(a), float32(b)))
}

// 値をmi~maの範囲で制限する
func clamp(a, mi, ma float32) float32 {
	return min(max(a, mi), ma)
}

func clampInt32(a, mi, ma int32) int32 {
	return int32(clamp(float32(a), float32(mi), float32(ma)))
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
	return vxs(x1-x0, y1-y0, px-x0, py-y0)
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
					neighbors = append(neighbors, neighbor)
				}
				fmt.Printf("neighbors: %v\n", neighbors)

				sectors = append(sectors, sector{floor: float32(floorHeight), ceil: float32(ceilHeight), vertex: vtx, npoints: uint32(len(vtx) - 1), neighbors: neighbors})
			case 'p': // player
				fmt.Sscanf(string(buf[len("player")+1:]), "%f %f %f	%d", &pl.where.x, &pl.where.y, &pl.angle, &pl.sector)
			}

		}
	}

	fmt.Printf("vert: %v\n", vert)
	fmt.Printf("player: %v\n", pl)
}

var renderer *sdl.Renderer = nil

// vline 最上部、中部、最下部で異なる色を使って水平方向に線を引く
func vline(x, y1, y2 int32, top, middle, bottom color.Color) {
	y1 = int32(clamp(float32(y1), 0, float32(H-1)))
	y2 = int32(clamp(float32(y2), 0, float32(H-1)))
	if y2 == y1 {
		r, g, b, a := middle.RGBA()
		renderer.SetDrawColor(uint8(r), uint8(g), uint8(b), uint8(a))
		renderer.DrawPoint(x, y1)
	} else if y2 > y1 {
		r, g, b, a := top.RGBA()
		renderer.SetDrawColor(uint8(r), uint8(g), uint8(b), uint8(a))
		renderer.DrawPoint(x, y1)

		r, g, b, a = middle.RGBA()
		renderer.SetDrawColor(uint8(r), uint8(g), uint8(b), uint8(a))
		renderer.DrawLine(x, y1, x, y2)

		r, g, b, a = bottom.RGBA()
		renderer.SetDrawColor(uint8(r), uint8(g), uint8(b), uint8(a))
		renderer.DrawPoint(x, y2)
	}
}

// MovePlayer プレイヤーの移動処理
func MovePlayer(dx, dy float32) {
	var px float32 = pl.where.x
	var py float32 = pl.where.y

	var sect *sector = &sectors[pl.sector]
	var vert []sdl.FPoint = sect.vertex

	for s := 0; s < int(sect.npoints); s++ {
		moveSector := sect.neighbors[s] >= 0 &&
			IntersectBox(px, py, px+dx, py+dy, vert[s].X, vert[s].Y, vert[s+1].X, vert[s+1].Y) &&
			PointSide(px+dx, py+dy, vert[s].X, vert[s].Y, vert[s+1].X, vert[s+1].Y) > 0
		if moveSector {
			pl.sector = sect.neighbors[s]
			break
		}
	}

	pl.where.x += dx
	pl.where.y += dy
	pl.anglesin = float32(math.Sin(float64(pl.angle)))
	pl.anglecos = float32(math.Cos(float64(pl.angle)))
}

// DrawScreen 描画処理
func DrawScreen() {
	const MaxQueue = 32
	type renderSector struct {
		sectorNo, sx1, sx2 int32
		ytop, ybottom      []int32
	}
	var queue []renderSector = []renderSector{}
	var renderedSector []int32 = []int32{}
	ybottom := make([]int32, W)
	for x := int32(0); x < W; x++ {
		ybottom[x] = H - 1
	}
	queue = append(queue, renderSector{sectorNo: pl.sector, sx1: 0, sx2: W - 1, ytop: make([]int32, W), ybottom: ybottom})

	for {
		if len(queue) == 0 {
			break
		}
		now := queue[0]
		queue = queue[1:]
		var sect *sector = &sectors[now.sectorNo]

		for _, rendered := range renderedSector {
			if rendered == now.sectorNo {
				continue
			}
		}
		renderedSector = append(renderedSector, now.sectorNo)
		for s := int32(0); s < int32(sect.npoints); s++ {
			vx1 := sect.vertex[s+1].X - pl.where.x
			vy1 := sect.vertex[s+1].Y - pl.where.y
			vx2 := sect.vertex[s].X - pl.where.x
			vy2 := sect.vertex[s].Y - pl.where.y

			pcos := pl.anglecos
			psin := pl.anglesin
			tx1 := vx1*psin - vy1*pcos
			tz1 := vx1*pcos + vy1*psin
			tx2 := vx2*psin - vy2*pcos
			tz2 := vx2*pcos + vy2*psin

			if tz1 <= 0 && tz2 <= 0 {
				// プレイヤーの正面に存在しない壁は描画しない
				continue
			}

			if tz1 <= 0 || tz2 <= 0 {
				// 見切れている場合はIntersectで求めたFOVとの交点まで描画を行う
				nearz := ParseToFloat32("1e-4")
				farz := float32(5)
				nearside := ParseToFloat32("1e-5")
				farside := ParseToFloat32("20.")
				i1x, i1y := Intersect(tx1, tz1, tx2, tz2, -nearside, nearz, -farside, farz)
				i2x, i2y := Intersect(tx1, tz1, tx2, tz2, nearside, nearz, farside, farz)

				if tz1 < nearz {
					if i1y > 0 {
						tx1 = i1x
						tz1 = i1y
					} else {
						tx1 = i2x
						tz1 = i2y
					}
				}
				if tz2 < nearz {
					if i1y > 0 {
						tx2 = i1x
						tz2 = i1y
					} else {
						tx2 = i2x
						tz2 = i2y
					}
				}
			}

			xscale1 := hFov / tz1
			yscale1 := vFov / tz1
			x1 := (W / 2) - int32(tx1*xscale1)
			xscale2 := hFov / tz2
			yscale2 := vFov / tz2
			x2 := (W / 2) - int32(tx2*xscale2)

			if x1 >= x2 || x2 < now.sx1 || x1 > now.sx2 {
				// 視界外は描画しない
				continue
			}

			yceil := sect.ceil - pl.where.z
			yfloor := sect.floor - pl.where.z

			neighbor := sect.neighbors[s]

			y1a := (H / 2) - int32(yceil*yscale1)
			y1b := (H / 2) - int32(yfloor*yscale1)
			y2a := (H / 2) - int32(yceil*yscale2)
			y2b := (H / 2) - int32(yfloor*yscale2)

			var nyceil float32 = 0
			var nyfloor float32 = 0
			if neighbor >= 0 {
				nyceil = sectors[neighbor].ceil - pl.where.z
				nyfloor = sectors[neighbor].floor - pl.where.z
			}
			ny1a := float32(H/2) - nyceil*yscale1
			ny1b := float32(H/2) - nyfloor*yscale1
			ny2a := float32(H/2) - nyceil*yscale2
			ny2b := float32(H/2) - nyfloor*yscale2

			beginx := max(float32(x1), float32(now.sx1))
			endx := min(float32(x2), float32(now.sx2))

			nextYTop := make([]int32, W)
			nextYBottom := make([]int32, W)
			for x := int32(beginx); x <= int32(endx); x++ {
				ya := (x-x1)*(y2a-y1a)/(x2-x1) + y1a
				cya := clampInt32(ya, now.ytop[x], now.ybottom[x])
				yb := (x-x1)*(y2b-y1b)/(x2-x1) + y1b
				cyb := clampInt32(yb, now.ytop[x], now.ybottom[x])

				vline(x, now.ytop[x], cya-1, color.RGBA{R: 128, G: 128, B: 128, A: 255}, color.RGBA{R: 212, G: 212, B: 212, A: 255}, color.RGBA{R: 128, G: 128, B: 128, A: 255})
				vline(x, cyb+1, now.ybottom[x], color.RGBA{R: 0, G: 0, B: 255, A: 255}, color.RGBA{R: 0, G: 0, B: 170, A: 255}, color.RGBA{R: 0, G: 0, B: 255, A: 255})

				if neighbor >= 0 {
					// vline(x, cya, cyb, color.RGBA{R: 0, G: 255, B: 0, A: 255}, color.RGBA{R: 255, G: 0, B: 0, A: 255}, color.RGBA{R: 0, G: 255, B: 0, A: 255})

					// 次のセクターより今のセクターの壁が高い/床が低い場合は壁を描画する
					nya := (x-x1)*int32(ny2a-ny1a)/(x2-x1) + int32(ny1a)
					cnya := clampInt32(nya, now.ytop[x], now.ybottom[x])
					nyb := (x-x1)*int32(ny2b-ny1b)/(x2-x1) + int32(ny1b)
					cnyb := clampInt32(nyb, now.ytop[x], now.ybottom[x])

					middleColor := color.RGBA{R: 170, G: 170, B: 170, A: 255}
					if x == x1 || x == x2 {
						middleColor = color.RGBA{R: 0, G: 0, B: 255, A: 255}
					}
					vline(x, cya, int32(cnya)-1, color.RGBA{R: 0, G: 0, B: 0, A: 255}, middleColor, color.RGBA{R: 0, G: 0, B: 0, A: 255})
					vline(x, int32(cnyb)+1, cyb, color.RGBA{R: 0, G: 0, B: 0, A: 255}, middleColor, color.RGBA{R: 0, G: 0, B: 0, A: 255})

					nextYTop[x] = clampInt32(maxInt32(cya, cnya), now.ytop[x], H-1)
					nextYBottom[x] = clampInt32(minInt32(cyb, cnyb), 0, now.ybottom[x])
				} else {
					middleColor := color.RGBA{R: 170, G: 170, B: 170, A: 255}
					if x == x1 || x == x2 {
						middleColor = color.RGBA{R: 0, G: 0, B: 255, A: 255}
					}
					vline(x, cya, cyb, color.RGBA{R: 0, G: 0, B: 0, A: 255}, middleColor, color.RGBA{R: 0, G: 0, B: 0, A: 255})
				}
			}

			if neighbor >= 0 && endx >= beginx && len(queue) < MaxQueue {
				queue = append(queue, renderSector{sectorNo: neighbor, sx1: int32(beginx), sx2: int32(endx), ytop: nextYTop, ybottom: nextYBottom})
			}
		}
	}
}

// DrawScreen2D 二次元座標系で描画
func DrawScreen2D() {
	err := renderer.SetDrawColor(0, 0, 0, 255)
	if err != nil {
		panic(err)
	}

	renderer.FillRect(&sdl.Rect{X: 0, Y: 0, W: 200, H: 300})

	for s := 0; s < len(sectors); s++ {
		err := renderer.SetDrawColor(255, 255, 255, 255)
		if err != nil {
			panic(err)
		}
		for v := 0; v < len(sectors[s].vertex)-1; v++ {
			vtx1 := sectors[s].vertex[v]
			vtx2 := sectors[s].vertex[v+1]
			renderer.DrawLineF(vtx1.X*10, vtx1.Y*10, vtx2.X*10, vtx2.Y*10)
		}
	}
	err = renderer.SetDrawColor(255, 0, 0, 255)
	if err != nil {
		panic(err)
	}

	for v := 0; v < len(sectors[pl.sector].vertex)-1; v++ {
		vtx1 := sectors[pl.sector].vertex[v]
		vtx2 := sectors[pl.sector].vertex[v+1]
		renderer.DrawLineF(vtx1.X*10, vtx1.Y*10, vtx2.X*10, vtx2.Y*10)
	}

	err = renderer.SetDrawColor(255, 0, 0, 255)
	if err != nil {
		panic(err)
	}
	plx := pl.where.x * 10
	ply := pl.where.y * 10

	renderer.DrawPointF(plx, ply)
	renderer.DrawLineF(plx, ply, plx+pl.anglecos*5, ply+pl.anglesin*5)
}

func cos32(angle float64) float32 {
	return float32(math.Cos(angle * math.Pi / 180))
}

func sin32(angle float64) float32 {
	return float32(math.Sin(angle * math.Pi / 180))
}

// ParseToFloat32 文字列をfloat32型に変換する
func ParseToFloat32(str string) float32 {
	f32, err := strconv.ParseFloat(str, 64)
	if err != nil {
		panic(err)
	}
	return float32(f32)
}

func main() {
	LoadData()

	if sdl.Init(uint32(sdl.INIT_EVERYTHING)) != nil {
		panic("Initialization failed: " + sdl.GetError().Error())
	}

	window, err := sdl.CreateWindow("portal rendering", 200, 30,
		W, H, (uint32)(sdl.WINDOW_SHOWN))
	if err != nil {
		panic(err)
	}

	_, err = sdl.ShowCursor(int(sdl.DISABLE))
	if err != nil {
		panic(err)
	}

	renderer, err = sdl.CreateRenderer(window, -1, 0)
	if err != nil {
		panic(err)
	}

	var wasd []bool = []bool{false, false, false, false}
	var yaw float32
	running := true
	//var ground bool = false
	var falling bool = true
	var moving bool = false
	var ducking bool = false
	for running {
		renderer.SetDrawColor(0, 0, 0, 0)
		renderer.Clear()

		DrawScreen()
		DrawScreen2D()

		renderer.Present()

		// 垂直方向の当たり判定
		var eyeheight float32
		if ducking {
			eyeheight = DuckHeight
		} else {
			eyeheight = EyeHeight
		}
		//ground = !falling
		if falling {
			pl.velocity.z -= 0.05
			nextz := pl.where.z + pl.velocity.z
			if pl.velocity.z < 0 && nextz < sectors[pl.sector].floor+eyeheight {
				// 地面に着地
				pl.where.z = sectors[pl.sector].floor + eyeheight
				pl.velocity.z = 0
				falling = false
				//ground = true
			} else if pl.velocity.z > 0 && nextz > sectors[pl.sector].ceil {
				// 天井に頭をぶつけた
				pl.velocity.z = 0
				falling = true
			}
			if falling {
				pl.where.z += pl.velocity.z
				moving = true
			}
		}

		// 水平方向の当たり判定
		if moving {
			var px float32 = pl.where.x
			var py float32 = pl.where.y
			var dx float32 = pl.velocity.x
			var dy float32 = pl.velocity.y

			var sect sector = sectors[pl.sector]
			var vert []sdl.FPoint = sect.vertex

			for s := 0; s < int(sect.npoints); s++ {
				if IntersectBox(px, py, px+dx, py+dy, vert[s].X, vert[s].Y, vert[s+1].X, vert[s+1].Y) && PointSide(px+dx, py+dy, vert[s].X, vert[s].Y, vert[s+1].X, vert[s+1].Y) > 0 {
					var hole_low float32
					if sect.neighbors[s] < 0 {
						hole_low = ParseToFloat32("9e9")
					} else {
						hole_low = max(sect.floor, sectors[sect.neighbors[s]].floor)
					}

					var hole_high float32
					if sect.neighbors[s] < 0 {
						hole_high = ParseToFloat32("-9e9")
					} else {
						hole_high = min(sect.ceil, sectors[sect.neighbors[s]].ceil)
					}

					if hole_high < pl.where.z+HeadMargin || hole_low > pl.where.z-eyeheight+KneeHeight {
						// 壁に衝突した場合は、移動方向を壁に沿わせる
						xd := vert[s+1].X - vert[s].X
						yd := vert[s+1].Y - vert[s].Y
						dx = xd * (dx*xd + yd*dy) / (xd*xd + yd*yd)
						dy = yd * (dx*xd + yd*dy) / (xd*xd + yd*yd)
						moving = false
					}
				}
			}
			MovePlayer(dx, dy)
			falling = true
		}

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch ev := event.(type) {
			case *sdl.QuitEvent:
				println("Quit")
				running = false
			case *sdl.KeyboardEvent:

				switch ev.Keysym.Scancode {
				case sdl.SCANCODE_W:
					wasd[0] = ev.Type == sdl.KEYDOWN
				case sdl.SCANCODE_A:
					wasd[1] = ev.Type == sdl.KEYDOWN
				case sdl.SCANCODE_S:
					wasd[2] = ev.Type == sdl.KEYDOWN
				case sdl.SCANCODE_D:
					wasd[3] = ev.Type == sdl.KEYDOWN
				case sdl.SCANCODE_ESCAPE:
					running = false
				}
			}
		}

		// マウスエイム
		x, y, _ := sdl.GetRelativeMouseState()
		pl.angle += float32(x) * 0.03
		yaw = clamp(yaw-float32(y)*0.05, -5, 5)
		pl.yaw = yaw - pl.velocity.z*0.5

		MovePlayer(0, 0)

		var moveVec sdl.FPoint = sdl.FPoint{}
		if wasd[0] {
			moveVec.X += pl.anglecos * 0.2
			moveVec.Y += pl.anglesin * 0.2
		}
		if wasd[1] {
			moveVec.X += pl.anglesin * 0.2
			moveVec.Y -= pl.anglecos * 0.2
		}
		if wasd[2] {
			moveVec.X -= pl.anglecos * 0.2
			moveVec.Y -= pl.anglesin * 0.2
		}
		if wasd[3] {
			moveVec.X -= pl.anglesin * 0.2
			moveVec.Y += pl.anglecos * 0.2
		}
		pushing := wasd[0] || wasd[1] || wasd[2] || wasd[3]
		acceleration := float32(0.2)
		if pushing {
			acceleration = 0.4
		}

		pl.velocity.x = pl.velocity.x*(1-acceleration) + moveVec.X*acceleration
		pl.velocity.y = pl.velocity.y*(1-acceleration) + moveVec.Y*acceleration

		if pushing {
			moving = true
		}

		sdl.Delay(10)
	}

	sdl.Quit()
}
