package tabMapview

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"maps"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/imui"
	packetcameraangles "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/cameraAnglesParser"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

type MapViewTab struct {
	Backend      backend.Backend[glfwbackend.GLFWWindowFlags]
	Rpl          *inspector.LoadedReplay
	Kills        *packetkill.PacketKillParser
	Ecs          *packetecs2.EntityManager
	Players      *packetslot.PacketSlotParser
	Paths        *packetmovement.PositionRetainerParser
	CameraAngles *packetcameraangles.PacketCameraAnglesParser

	TankMapsPath string
	DataminePath string

	initErr         error
	tankmapTextureW int
	tankmapTextureH int
	tankmapTexture  *imgui.TextureRef
	rLevel          string
	rLevelSettings  string
	rBattleType     string
	rAreas          map[string]AreaDef
	rMission        MissionDef
	rOffsets        levelDef
	rMainAreaName   string
	rImageArea      image.Rectangle
	rCaps           map[int]AreaDef
	rCoordScaleX    float64
	rCoordScaleZ    float64

	highlightPath     uint64
	hoveredPath       uint64
	hoveredPathRender bool

	stubNotFound int
	stubIdx      int32

	imOutSize imgui.Vec2
	imOutSp   imgui.Vec2

	pbCurrentTime     uint32
	pbStartedRealTime time.Time
	pbStartedGameTime uint32
	pbIsPlaying       bool
	pbPlaybackSpeed   float32
	pbTrailDuration   uint32
	pbButtonSize      float32

	cacheLatestPlaybackTime  uint32
	cacheLatestPlaybackIndex []int
	cachePathEidsSorted      []uint64
}

func (tab *MapViewTab) Name() string {
	return "MapView"
}

func (tab *MapViewTab) Init() {
	if tab.Rpl == nil {
		tab.initErr = errors.New("rpl is nil")
		return
	}
	tab.rLevel = string(bytes.Trim(tab.Rpl.Header.Raw_Level[:], "\x00"))
	tab.rLevelSettings = string(bytes.Trim(tab.Rpl.Header.Raw_LevelSettings[:], "\x00"))
	tab.rBattleType = string(bytes.Trim(tab.Rpl.Header.Raw_BattleType[:], "\x00"))

	tankmapImage, err := levelToTankmap(tab.TankMapsPath, tab.rLevel)
	if err != nil {
		tankmapImage = image.NewRGBA(image.Rect(0, 0, 2048, 2048))
		// tab.initErr = fmt.Errorf("levelToTankmap: %w", err)
		// return
	}
	tab.tankmapTextureW = tankmapImage.Rect.Dx()
	tab.tankmapTextureH = tankmapImage.Rect.Dy()
	tex := tab.Backend.CreateTextureRgba(tankmapImage, tankmapImage.Rect.Dx(), tankmapImage.Rect.Dy())
	tab.tankmapTexture = &tex

	mission, err := missionLoad(tab.DataminePath, tab.rLevelSettings)
	if err != nil {
		tab.initErr = fmt.Errorf("missionLoad: %w", err)
		return
	}
	tab.rMission = *mission

	areas := map[string]AreaDef{}
	if mission != nil && mission.Areas != nil {
		areas = mission.Areas
	}
	tab.rAreas = areas

	offsets, err := getLevelCoords(tab.DataminePath, tab.rLevel)
	if err != nil {
		tab.initErr = fmt.Errorf("getLevelCoords: %w", err)
		return
	}
	tab.rOffsets = *offsets

	mainArea, mainAreaName, err := findMainBattleArea(areas, tab.rBattleType, tab.Rpl.Header.Difficulty)
	if err != nil {
		tab.initErr = fmt.Errorf("findMainBattleArea: %w", err)
		return
	}
	tab.rMainAreaName = mainAreaName
	mainAreaX, mainAreaZ, mainAreaW, mainAreaH := calcDrawArea(offsets, mainArea, false)
	tab.rImageArea = image.Rect(int(mainAreaX), int(mainAreaZ), int(mainAreaX+mainAreaW), int(mainAreaZ+mainAreaH))

	caps, err := findCaps(areas, tab.rBattleType, tab.Rpl.Header.Difficulty)
	if err != nil {
		caps = map[int]AreaDef{}
	}
	tab.rCaps = caps

	tab.rCoordScaleX = math.Abs(offsets.TankMapCoord1[0]-offsets.TankMapCoord0[0]) / 2048
	tab.rCoordScaleZ = math.Abs(offsets.TankMapCoord1[1]-offsets.TankMapCoord0[1]) / 2048

	tab.pbCurrentTime = tab.Rpl.Packets[0].CurrentTime
	tab.pbPlaybackSpeed = 1
	tab.pbTrailDuration = 30000
	tab.pbButtonSize = max(imgui.CalcTextSize("pause").X, imgui.CalcTextSize("play").X) + 10

	type pair struct {
		k uint64
		v uint32
	}
	sortPairs := []pair{}
	for k, v := range tab.Paths.Paths {
		if len(v) == 0 {
			continue
		}
		sortPairs = append(sortPairs, pair{
			k: k,
			v: v[0].Time,
		})
	}
	slices.SortFunc(sortPairs, func(a, b pair) int {
		r := int(a.v) - int(b.v)
		if r != 0 {
			return r
		}
		return int(a.k) - int(b.k)
	})
	tab.cachePathEidsSorted = make([]uint64, len(sortPairs))
	for i, v := range sortPairs {
		tab.cachePathEidsSorted[i] = v.k
	}
	tab.cacheLatestPlaybackIndex = make([]int, len(sortPairs))
}

func (tab *MapViewTab) Run() {
	if imgui.BeginChildStrV("map view tab content", imgui.ContentRegionAvail(), 0, imgui.WindowFlagsHorizontalScrollbar) {
		tab.RunContent()
	}
	imgui.EndChild()
}

func (tab *MapViewTab) RunContent() {
	tab.RunControls()
	padY := imgui.CurrentStyle().FramePadding().Y
	avail := imgui.ContentRegionAvail()
	tab.imOutSize = imgui.Vec2{X: min(avail.X, avail.Y), Y: min(avail.X, avail.Y) - padY}
	tab.imOutSp = imgui.CursorScreenPos()
	tab.imOutSp.Y += padY
	if tab.initErr != nil {
		imgui.PushTextWrapPos()
		imgui.TextUnformatted("init error: " + tab.initErr.Error())
		imgui.PopTextWrapPos()
		if imgui.Button("ignore") {
			tab.initErr = nil
		}
	} else {
		tab.DrawView()
	}

	imgui.Dummy(tab.imOutSize)
	imgui.SameLine()
	if imgui.BeginChildStr("map side view") {
		if imgui.BeginTabBar("side tabs") {
			if imgui.BeginTabItem("Paths") {
				tab.runPaths()
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Info") {
				tab.runGeneral()
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Areas") {
				tab.runAreas()
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Offsets") {
				tab.runOffsets()
				imgui.EndTabItem()
			}
			imgui.EndTabBar()
		}
	}
	imgui.EndChild()
}

func (tab *MapViewTab) RunControls() {
	replayTimeStart := tab.Rpl.Packets[0].CurrentTime
	replayTimeEnd := tab.Rpl.Packets[len(tab.Rpl.Packets)-1].CurrentTime
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprint((time.Duration(tab.pbCurrentTime) * time.Millisecond).String()))
	imgui.SameLineV(90, -1)
	if tab.pbIsPlaying {
		if imgui.ButtonV("pause", imgui.Vec2{X: tab.pbButtonSize, Y: 0}) {
			tab.pbIsPlaying = false
		}
	} else {
		if imgui.ButtonV("play", imgui.Vec2{X: tab.pbButtonSize, Y: 0}) {
			tab.pbIsPlaying = true
			tab.pbStartedRealTime = time.Now()
			tab.pbStartedGameTime = tab.pbCurrentTime
		}
	}
	if tab.pbIsPlaying {
		tab.pbCurrentTime = tab.pbStartedGameTime + uint32(float32(time.Since(tab.pbStartedRealTime).Milliseconds())*tab.pbPlaybackSpeed)
		if tab.pbCurrentTime >= replayTimeEnd {
			tab.pbIsPlaying = false
			tab.pbCurrentTime = replayTimeEnd
		}
	}
	p := float32(uint32(tab.pbCurrentTime)-replayTimeStart) / float32(replayTimeEnd-replayTimeStart)
	if tab.pbCurrentTime < replayTimeStart {
		p = 0
	}
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X - 200)
	if imgui.SliderFloat("##timeslider", &p, 0, 1) {
		tab.pbCurrentTime = replayTimeStart + uint32(p*float32(replayTimeEnd-replayTimeStart))
		tab.pbStartedRealTime = time.Now()
		tab.pbStartedGameTime = tab.pbCurrentTime
	}
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	if imgui.SliderFloat("##speed", &tab.pbPlaybackSpeed, 0, 16) {
		tab.pbStartedRealTime = time.Now()
		tab.pbStartedGameTime = tab.pbCurrentTime
	}
}

func (tab *MapViewTab) DrawView() {
	dl := imgui.WindowDrawList()

	uv0 := imgui.Vec2{X: float32(tab.rImageArea.Min.X) / float32(tab.tankmapTextureW), Y: float32(tab.rImageArea.Min.Y) / float32(tab.tankmapTextureH)}
	uv1 := imgui.Vec2{X: float32(tab.rImageArea.Max.X) / float32(tab.tankmapTextureW), Y: float32(tab.rImageArea.Max.Y) / float32(tab.tankmapTextureH)}
	dlImage(dl, *tab.tankmapTexture, tab.imOutSp, tab.imOutSize, uv0, uv1)

	sw := float64(tab.imOutSize.X / float32(tab.rImageArea.Dx()))
	sh := float64(tab.imOutSize.Y / float32(tab.rImageArea.Dy()))
	showEverything := tab.pbCurrentTime <= tab.Rpl.Packets[0].CurrentTime
	for _, eid := range tab.cachePathEidsSorted {
		path := tab.Paths.Paths[eid]
		eid2 := ((uint64(uint64(eid)&0xff) << uint64(0x16)) | (uint64(eid) >> uint64(0x8))) & 0x7FF
		eid2 = uint64(packetecs2.EntityID(uint32(eid2)).Index())
		e := tab.Ecs.Entities[uint32(eid2)]
		if e == nil {
			continue
		}
		foundDeath := -1
		for i := range tab.Kills.Kills {
			if tab.Kills.Kills[i].ResolvedVictim != e {
				continue
			}
			foundDeath = i
			break
		}
		if showEverything {
			coords := imgui.Vec2{}
			deathCoords := imgui.Vec2{}
			deathCoordsSet := false
			// killerCoords := imgui.Vec2{}
			// killerCoordsSet := false
			for _, pos := range path {
				x := (((float64(pos.X) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw
				z := ((2048 - (float64(pos.Z)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh
				coords = tab.imOutSp.Add(imgui.Vec2{X: float32(x), Y: float32(z)})
				dl.PathLineToMergeDuplicate(coords)
				if foundDeath != -1 && pos.Time >= tab.Kills.Kills[foundDeath].CurrentTime && !deathCoordsSet {
					deathCoords = coords
					deathCoordsSet = true
					// killerPos := tab.Kills.Kills[foundDeath].ResolvedKillerPosition
					// if killerPos != nil {
					// 	killerCoords = imgui.Vec2{
					// 		X: float32((((float64(killerPos.X) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw),
					// 		Y: float32((((float64(killerPos.Z) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw),
					// 	}
					// 	killerCoordsSet = true
					// }
				}
			}
			dl.PathStroke(0xFFFFFFFF)
			dl.AddCircleFilled(coords, 3, 0xFF00FF00)
			if deathCoordsSet {
				dl.AddCircleFilled(deathCoords, 6, 0xFF0000FF)
			}
			// if killerCoordsSet {
			// 	dl.AddCircleFilled(killerCoords, 11, 0x880000FF)
			// 	dl.AddCircleFilled(killerCoords, 13, 0x880000FF)
			// }
			continue
		}
		if path[0].Time > tab.pbCurrentTime {
			continue
		}
		if path[len(path)-1].Time+tab.pbTrailDuration <= tab.pbCurrentTime {
			continue
		}
		if foundDeath != -1 && tab.Kills.Kills[foundDeath].CurrentTime+tab.pbTrailDuration <= tab.pbCurrentTime {
			continue
		}
		coords := imgui.Vec2{}
		deathCoords := imgui.Vec2{}
		deathCoordsSet := false
		killerCoords := imgui.Vec2{}
		killerCoordsSet := false
		for _, pos := range path {
			if pos.Time >= tab.pbCurrentTime || pos.Time <= tab.pbCurrentTime-tab.pbTrailDuration {
				continue
			}
			x := (((float64(pos.X) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw
			z := ((2048 - (float64(pos.Z)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh
			coords = tab.imOutSp.Add(imgui.Vec2{X: float32(x), Y: float32(z)})
			dl.PathLineToMergeDuplicate(coords)
			if foundDeath != -1 && pos.Time >= tab.Kills.Kills[foundDeath].CurrentTime && !deathCoordsSet {
				deathCoords = coords
				deathCoordsSet = true
				killerPos := tab.Kills.Kills[foundDeath].ResolvedKillerPosition
				if killerPos != nil {
					killerCoords = tab.imOutSp.Add(imgui.Vec2{
						X: float32((((float64(killerPos.X) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw),
						Y: float32(((2048 - (float64(killerPos.Z)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh),
					})
					killerCoordsSet = true
				}
			}
		}
		dl.PathStroke(0xFFFFFFFF)
		stubVals := tab.CameraAngles.Data[eid]
		if stubVals == nil {
			tab.stubNotFound++
		} else {
			for i := range stubVals {
				if stubVals[i].CurrentTime < tab.pbCurrentTime {
					continue
				}
				yaw := stubVals[i].Possible_gun_cirlce_ang[1] - math.Pi/2
				//yaw2 := stubVals[i].Possible_looking_ang[1] - math.Pi/2
				dl.AddLine(coords, coords.Add(imgui.Vec2{
					X: float32(math.Cos(float64(yaw)) * 15),
					Y: float32(math.Sin(float64(yaw)) * 15),
				}), 0xFFFF0000)
				//dl.AddLine(coords, coords.Add(imgui.Vec2{
				//	X: float32(math.Cos(float64(yaw2)) * 15),
				//	Y: float32(math.Sin(float64(yaw2)) * 15),
				//}), 0xAAAA0000)
				break
			}
		}
		if deathCoordsSet {
			a := uint32(255-255*float32(tab.pbCurrentTime-tab.Kills.Kills[foundDeath].CurrentTime)/float32(tab.pbTrailDuration)) << 24
			dl.AddCircleFilled(deathCoords, 6, 0x000000AA|a)
			if killerCoordsSet {
				// dl.AddCircle(killerCoords, 11, 0x000000FF|a)
				// dl.AddCircle(killerCoords, 13, 0x000000FF|a)
				dl.AddLine(deathCoords, killerCoords, 0x000000FF|a)
				midpoint := deathCoords
				for range 4 {
					midpoint = midpoint.Add(killerCoords).Div(2)
					dl.AddLine(midpoint, killerCoords, 0x0000FFFF|a)
				}
			}
		} else {
			dl.AddCircleFilled(coords, 3, 0xFF00FF00)
		}
	}
	hpath := tab.Paths.Paths[tab.highlightPath]
	if hpath != nil {
		for _, pos := range hpath {
			x := (((float64(pos.X) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw
			z := ((2048 - (float64(pos.Z)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh
			coords := imgui.Vec2{X: float32(x), Y: float32(z)}
			dl.PathLineTo(tab.imOutSp.Add(coords))
		}
		dl.PathStrokeV(0xAA0000FF, 0, 3)
	}
	hpath = tab.Paths.Paths[tab.hoveredPath]
	if hpath != nil && tab.hoveredPathRender {
		for _, pos := range hpath {
			x := (((float64(pos.X) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw
			z := ((2048 - (float64(pos.Z)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh
			coords := imgui.Vec2{X: float32(x), Y: float32(z)}
			dl.PathLineTo(tab.imOutSp.Add(coords))
		}
		dl.PathStrokeV(0xAA0000FF, 0, 3)
	}

	// for _, k := range tab.Kills.Kills {
	// 	x := (((float64(k.ResolvedVictimPositionX) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw
	// 	z := ((2048 - (float64(k.ResolvedVictimPositionZ)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh
	// 	coords := imgui.Vec2{X: float32(x), Y: float32(z)}
	// 	dl.AddCircle(tab.imOutSp.Add(coords), 11, 0xFF0000FF)
	// 	dl.AddCircle(tab.imOutSp.Add(coords), 13, 0xFF0000FF)
	// 	x = (((float64(k.ResolvedKillerPositionX) - tab.rOffsets.TankMapCoord0[0]) / tab.rCoordScaleX) - float64(tab.rImageArea.Min.X)) * sw
	// 	z = ((2048 - (float64(k.ResolvedKillerPositionZ)-tab.rOffsets.TankMapCoord0[1])/tab.rCoordScaleZ) - float64(tab.rImageArea.Min.Y)) * sh
	// 	coords = imgui.Vec2{X: float32(x), Y: float32(z)}
	// 	dl.AddCircle(tab.imOutSp.Add(coords), 11, 0xFF00FF00)
	// 	dl.AddCircle(tab.imOutSp.Add(coords), 13, 0xFF00FF00)
	// }
}

func (tab *MapViewTab) runGeneral() {
	imgui.TextUnformatted(fmt.Sprint("Playback: ", tab.pbIsPlaying))
	imgui.TextUnformatted(fmt.Sprint("Playback current time: ", tab.pbCurrentTime))
	imgui.TextUnformatted(fmt.Sprint("Playback started real time: ", tab.pbStartedRealTime))
	imgui.TextUnformatted(fmt.Sprint("Playback started game time: ", tab.pbStartedGameTime))
	imgui.TextUnformatted("Level: " + tab.rLevel)
	imgui.TextUnformatted("Level settings: " + tab.rLevelSettings)
	imgui.TextUnformatted("Battle type: " + tab.rBattleType)
	imgui.TextUnformatted("Main area: " + tab.rMainAreaName)
	imgui.TextUnformatted(fmt.Sprint("Offsets: ", tab.rOffsets))
	imgui.TextUnformatted(fmt.Sprint("Image area: ", tab.rImageArea))
	imgui.TextUnformatted(fmt.Sprint("Image width: ", tab.rImageArea.Dx()))
	imgui.TextUnformatted(fmt.Sprint("Image height: ", tab.rImageArea.Dy()))
	imgui.TextUnformatted(fmt.Sprint("Coord scale X: ", tab.rCoordScaleX))
	imgui.TextUnformatted(fmt.Sprint("Coord scale Z: ", tab.rCoordScaleZ))
	imgui.TextUnformatted(fmt.Sprint("Sp: ", tab.imOutSp))
	imgui.TextUnformatted(fmt.Sprint("Out size: ", tab.imOutSize))
	imgui.TextUnformatted(fmt.Sprint("Aspect: ", float32(tab.rImageArea.Dx())/float32(tab.rImageArea.Dy())))
	imgui.Separator()
	imgui.TextUnformatted(fmt.Sprint("stub not found: ", tab.stubNotFound))
	tab.stubNotFound = 0
	imgui.SliderInt("stub idx", &tab.stubIdx, 0, 9)
}

func (tab *MapViewTab) runPaths() {
	flags := imgui.TableFlagsBorders | imgui.TableFlagsResizable | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsNoHostExtendX
	if imgui.BeginTableV("paths", 7, flags, imgui.Vec2{}, 0) {
		imgui.TableSetupColumn("eid")
		imgui.TableSetupColumn("count")
		imgui.TableSetupColumn("Tfirst")
		imgui.TableSetupColumn("Tlast")
		imgui.TableSetupColumn("player")
		imgui.TableSetupColumn("model")
		imgui.TableSetupColumn("died")
		imgui.TableHeadersRow()
		type pair struct {
			k uint64
			v uint32
		}
		isAnythingHovered := false
		for _, eid := range tab.cachePathEidsSorted {
			path := tab.Paths.Paths[eid]
			eid2 := ((uint64(uint64(eid)&0xff) << uint64(0x16)) | (uint64(eid) >> uint64(0x8))) & 0x7FF
			eid2 = uint64(packetecs2.EntityID(uint32(eid2)).Index())

			imgui.TableNextRow()
			isRowSelected := tab.highlightPath == eid
			if isRowSelected {
				imgui.TableSetBgColor(imgui.TableBgTargetRowBg1, 0x22FFFFFF)
			}

			imgui.TableNextColumn()
			if imgui.SelectableBoolV(strconv.FormatUint(eid, 10), isRowSelected, imgui.SelectableFlagsSpanAllColumns, imgui.Vec2{}) {
				tab.highlightPath = eid
			}
			if imgui.IsItemHovered() {
				isAnythingHovered = true
				tab.hoveredPath = eid
			}

			imgui.TableNextColumn()
			imgui.TextUnformatted(strconv.FormatInt(int64(len(path)), 10))

			imgui.TableNextColumn()
			imgui.TextUnformatted((time.Duration(path[0].Time) * time.Millisecond).String())

			imgui.TableNextColumn()
			imgui.TextUnformatted((time.Duration(path[len(path)-1].Time) * time.Millisecond).String())

			imgui.TableNextColumn()
			e := tab.Ecs.Entities[uint32(eid2)]
			if e == nil {
				imgui.TextUnformatted("unresolved entity")
				continue
			}
			playerid, ok := packetecs2.GetObjectData[int32](&e.Data, "unit__playerId")
			if !ok {
				imgui.TextUnformatted("unresolved unit__playerId")
				if imgui.IsItemHoveredV(imgui.HoveredFlagsForTooltip) {
					if imgui.BeginTooltip() {
						imgui.TextUnformatted(spew.Sdump(e.Data))
						imgui.EndTooltip()
					}
				}
				continue
			}
			if playerid < 0 || playerid >= 255 {
				imgui.TextUnformatted("unit__playerId oob")
				continue
			}
			player := tab.Players.Players[playerid]
			if player == nil {
				imgui.TextUnformatted("nil player")
				continue
			}
			imgui.TextUnformatted(player.Uid.Name)

			imgui.TableNextColumn()
			modelName, ok := packetecs2.GetObjectData[string](&e.Data, "unit__className")
			if !ok {
				imgui.TextUnformatted("unresolved unit__className")
				continue
			}
			imgui.TextUnformatted(modelName)

			imgui.TableNextColumn()
			uid, ok := packetecs2.GetObjectData[int32](&e.Data, "uid")
			if !ok {
				imgui.TextUnformatted("unresolved uid")
			}
			killedAt := uint32(0)
			for _, kill := range tab.Kills.Kills {
				if kill.VictimUid == uint16(uid) {
					killedAt = kill.CurrentTime
					break
				}
			}
			imgui.TextUnformatted((time.Duration(killedAt) * time.Millisecond).String())
		}
		tab.hoveredPathRender = isAnythingHovered
	}
	imgui.EndTable()
}

func (tab *MapViewTab) runAreas() {
	for _, k := range slices.Sorted(maps.Keys(tab.rAreas)) {
		v := tab.rAreas[k]
		if imgui.TreeNodeStr(fmt.Sprintf("%q %q", k, v.Type)) {
			for i2, v2 := range v.TM {
				imgui.TextUnformatted(fmt.Sprintf("%d %v", i2, v2))
			}
			imgui.TreePop()
		}
	}
}

func (tab *MapViewTab) runOffsets() {
	s := spew.Sprint(tab.rOffsets)
	imgui.InputTextMultiline("##basictext", &s, imgui.ContentRegionAvail(), imgui.InputTextFlagsReadOnly, imui.ImEmptyInputCallback)
}

func (tab *MapViewTab) Close() {
	if tab.tankmapTexture != nil {
		tab.Backend.DeleteTexture(*tab.tankmapTexture)
	}
}

func ImSliderVec2(label string, v *imgui.Vec2, min, max float32) bool {
	sh := [2]float32{v.X, v.Y}
	ret := imgui.SliderFloat2(label, &sh, min, max)
	if ret {
		v.X = sh[0]
		v.Y = sh[1]
	}
	return ret
}

func dlImage(dl *imgui.DrawList, ref imgui.TextureRef, sp, size, uv0, uv1 imgui.Vec2) {
	dl.AddImageQuadV(ref,
		sp, sp.Add(imgui.Vec2{X: size.X}), sp.Add(imgui.Vec2{X: size.X, Y: size.Y}), sp.Add(imgui.Vec2{Y: size.Y}),
		uv0, imgui.Vec2{X: uv1.X, Y: uv0.Y}, imgui.Vec2{X: uv1.X, Y: uv1.Y}, imgui.Vec2{X: uv0.X, Y: uv1.Y}, 0xFFFFFFFF)
}
