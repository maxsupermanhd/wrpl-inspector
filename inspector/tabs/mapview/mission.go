package tabMapview

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type MissionDef struct {
	Areas   map[string]AreaDef `json:"areas"`
	Imports map[string]any     `json:"imports"`
}

func missionLoad(dataminePath, missionPath string) (*MissionDef, error) {
	if missionPath == "" {
		return nil, nil
	}
	missionBytes, err := os.ReadFile(filepath.Join(dataminePath, strings.ToLower(`mis.vromfs.bin_u/`+missionPath+"x")))
	if err != nil {
		return nil, err
	}
	var mission *MissionDef
	err = json.Unmarshal(missionBytes, &mission)
	if err != nil {
		return nil, err
	}
	if mission == nil {
		return nil, errors.New("json unmarshal nil")
	}
	if len(mission.Imports) == 0 {
		return mission, nil
	}
	if mission.Areas == nil {
		mission.Areas = map[string]AreaDef{}
	}
	imp, ok := mission.Imports["import_record"].(map[string]any)
	if ok {
		fp, ok := imp["file"].(string)
		if !ok {
			return nil, errors.New("import record has no file path")
		}
		impMis, err := missionLoad(dataminePath, fp)
		if err != nil {
			return nil, fmt.Errorf("importing %q: %w", fp, err)
		}
		if impMis != nil && impMis.Areas != nil {
			maps.Insert(mission.Areas, maps.All(impMis.Areas))
		}
	} else {
		imports, ok := mission.Imports["import_record"].([]any)
		if !ok {
			return nil, errors.New("import record is not an object or an array")
		}
		for i, impAny := range imports {
			imp, ok = impAny.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("import record %d is not an object", i)
			}
			fp, ok := imp["file"].(string)
			if !ok {
				return nil, fmt.Errorf("import record %d has no file path", i)
			}
			impMis, err := missionLoad(dataminePath, fp)
			if err != nil {
				return nil, fmt.Errorf("importing %q: %w", fp, err)
			}
			if impMis != nil && impMis.Areas != nil {
				maps.Insert(mission.Areas, maps.All(impMis.Areas))
			}
		}
	}
	return mission, nil
}

type levelDef struct {
	TankMapCoord0 []float64 `json:"tankMapCoord0"`
	TankMapCoord1 []float64 `json:"tankMapCoord1"`
}

func getLevelCoords(dataminePath, levelPath string) (*levelDef, error) {
	levelPath = filepath.Join(dataminePath, `aces.vromfs.bin_u/`+strings.TrimSuffix(levelPath, ".bin")+".blkx")
	levelBytes, err := os.ReadFile(levelPath)
	if err != nil {
		return nil, err
	}
	var level levelDef
	err = json.Unmarshal(levelBytes, &level)
	if err != nil {
		return nil, fmt.Errorf("parsing level %q: %w", levelPath, err)
	}
	return &level, nil
}

type AreaDef struct {
	Type string      `json:"type"`
	TM   [][]float32 `json:"tm"`
}

func findCaps(areas map[string]AreaDef, bttlType string, difficulty byte) (map[int]AreaDef, error) {
	if areas == nil {
		return nil, errors.New("nil areas")
	}
	lastUnderscore := strings.LastIndex(bttlType, "_")
	if lastUnderscore == -1 {
		return nil, fmt.Errorf("underscore not found in %q", bttlType)
	}
	bttlType = strings.ToLower(bttlType[lastUnderscore:])
	bttlType = strings.Trim(bttlType, "_")
	difficultyStr := snailDifficultyToStr(difficulty)
	ret := map[int]AreaDef{}
	var found *AreaDef
	switch bttlType {
	case "bttl":
		found = findAreaInList(areas, []string{
			"bttl_t1_capture_area_" + difficultyStr,
			"bttl_t1_capture_area_" + "arcade",
		})
		if found != nil {
			ret[0] = *found
		}
		found = findAreaInList(areas, []string{
			"bttl_t2_capture_area_" + difficultyStr,
			"bttl_t2_capture_area_" + "arcade",
		})
		if found != nil {
			ret[1] = *found
		}
	case "dom":
		found = findAreaInList(areas, []string{
			"dom_capture_area_01_" + difficultyStr,
			"dom_capture_area_01_" + "arcade",
		})
		if found != nil {
			ret[0] = *found
		}
		found = findAreaInList(areas, []string{
			"dom_capture_area_02_" + difficultyStr,
			"dom_capture_area_02_" + "arcade",
		})
		if found != nil {
			ret[1] = *found
		}
		found = findAreaInList(areas, []string{
			"dom_capture_area_03_" + difficultyStr,
			"dom_capture_area_03_" + "arcade",
		})
		if found != nil {
			ret[2] = *found
		}
	case "conq1":
		found = findAreaInList(areas, []string{
			"conq_capture_area_01_" + difficultyStr,
			"conq_capture_area_01_" + "arcade",
		})
		if found != nil {
			ret[0] = *found
		}
	case "conq2":
		found = findAreaInList(areas, []string{
			"conq_capture_area_02_" + difficultyStr,
			"conq_capture_area_02_" + "arcade",
		})
		if found != nil {
			ret[0] = *found
		}
	case "conq3":
		found = findAreaInList(areas, []string{
			"conq_capture_area_03_" + difficultyStr,
			"conq_capture_area_03_" + "arcade",
		})
		if found != nil {
			ret[0] = *found
		}
	case "conq4":
		found = findAreaInList(areas, []string{
			"conq_capture_area_04_" + difficultyStr,
			"conq_capture_area_04_" + "arcade",
		})
		if found != nil {
			ret[0] = *found
		}
	}
	return ret, nil
}

func findAreaInList(areas map[string]AreaDef, search []string) *AreaDef {
	for _, k := range search {
		ret, ok := areas[k]
		if ok {
			return &ret
		}
	}
	return nil
}

func findMainBattleArea(areas map[string]AreaDef, bttlType string, difficulty byte) (*AreaDef, string, error) {
	if areas == nil {
		return nil, "", errors.New("findMainBattleArea: nil areas")
	}
	lastUnderscore := strings.LastIndex(bttlType, "_")
	if lastUnderscore == -1 {
		return nil, "", fmt.Errorf("underscore not found in %q", bttlType)
	}
	bttlType = strings.ToLower(bttlType[lastUnderscore:])
	bttlType = strings.Trim(bttlType, "_")
	bttlType = strings.TrimRight(bttlType, "01234")
	difficultyStr := ""
	switch difficulty >> 2 & 3 {
	case 2:
		difficultyStr = "hardcore"
	case 1:
		difficultyStr = "realistic"
	case 0:
		difficultyStr = "arcade"
	// case 186:
	// 	difficultyStr = "hardcore"
	// case 181:
	// 	difficultyStr = "realistic"
	// case 117:
	// 	difficultyStr = "realistic"
	// case 48:
	// 	difficultyStr = "arcade"
	default:
		return nil, "", fmt.Errorf("findMainBattleArea: unknown difficulty %d (%d)", difficulty, difficulty>>2&3)
	}
	tryNames := []string{
		bttlType + "_battle_area_" + difficultyStr,
		bttlType + "_battle_area_" + "arcade",
		"bttl" + "_battle_area_" + difficultyStr,
		"bttl" + "_battle_area_" + "arcade",
		"dom" + "_battle_area_" + difficultyStr,
		"dom" + "_battle_area_" + "arcade",
		"battle_area",
	}
	for _, k := range tryNames {
		ret, ok := areas[k]
		if ok {
			return &ret, k, nil
		}
	}
	as := []string{}
	for an := range areas {
		as = append(as, an)
	}
	return nil, "", fmt.Errorf("findMainBattleArea: main battle area (tried %v) not found (difficulty %q) (blk %q) (out of %+#v)", tryNames, difficultyStr, bttlType, as)
}

func calcDrawArea(offsets *levelDef, area *AreaDef, makeSquare bool) (x, z, w, h float64) {
	areaSizeX := math.Abs(offsets.TankMapCoord1[0] - offsets.TankMapCoord0[0])
	areaSizeZ := math.Abs(offsets.TankMapCoord1[1] - offsets.TankMapCoord0[1])
	coordScaleX := areaSizeX / 2048
	coordScaleZ := areaSizeZ / 2048
	x = float64(area.TM[3][0]) - offsets.TankMapCoord0[0]
	z = float64(area.TM[3][2]) - offsets.TankMapCoord0[1]
	x /= coordScaleX
	z /= coordScaleZ
	z = 2048 - z
	w = float64(area.TM[0][0] + area.TM[0][2])
	h = float64(area.TM[2][0] + area.TM[2][2])
	w /= coordScaleX
	h /= coordScaleZ
	if area.Type == "Cylinder" {
		w *= 2
		h *= 2
	}
	if (w > 0 && h < 0) || (w < 0 && h > 0) {
		tmp := w
		w = h
		h = tmp
	}
	if makeSquare {
		s := max(math.Abs(w), math.Abs(h))
		if w < 0 {
			w = -s
		} else {
			w = s
		}
		if h < 0 {
			h = -s
		} else {
			h = s
		}
	}
	x = x - w/2
	z = z - h/2
	return
}

func snailDifficultyToStr(difficulty byte) string {
	return []string{"arcade", "realistic", "hardcore"}[(difficulty>>2)&3]
}
