package resultsui

import (
	"encoding/json"
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl"
)

type ResultsTab struct {
	raw     map[string]any
	rawJSON string
	rawSPEW string
}

func NewResultsTab(resultsBlkBytes []byte) (*ResultsTab, error) {
	if len(resultsBlkBytes) == 0 {
		return &ResultsTab{
			raw: map[string]any{},
		}, nil
	}
	raw, err := wrpl.ParseBlk(resultsBlkBytes)
	if err != nil {
		return nil, err
	}

	rawJSONb, err := json.MarshalIndent(raw, "", "\t")
	rawJSON := string(rawJSONb)
	if err != nil {
		rawJSON = err.Error()
	}

	return &ResultsTab{
		raw:     raw,
		rawJSON: rawJSON,
		rawSPEW: spew.Sdump(raw),
	}, nil
}

func (tab *ResultsTab) Name() string {
	return "Results"
}

func (tab *ResultsTab) Init() {
}

/*

	"76484480": {
		"crafts_info": {
			"__array": true,
			"array0": {
				"mrank": 20,
				"name": "us_m103",
				"rank": 5,
				"rankUnused": false
			},
			"array1": {
				"mrank": 20,
				"name": "us_t32e1",
				"rank": 5,
				"rankUnused": true
			},
			"array2": {
				"mrank": 19,
				"name": "us_m163_vulcan",
				"rank": 5,
				"rankUnused": true
			},
			"array3": {
				"mrank": 16,
				"name": "p-51h-5_na",
				"rank": 4,
				"rankUnused": true
			},
			"array4": {
				"mrank": 14,
				"name": "am_1_mauler",
				"rank": 4,
				"rankUnused": true
			}
		},
		"mrank": 16
	},

	{
		"deaths": 2,
		"aiGroundKills": 0,
		"aiKills": 0,
		"aiNavalKills": 0,
		"groundKills": 0,
		"humanKills": 0,
		"kills": 0,
		"navalKills": 0,
		"teamKills": 0,
		"assists": 1,
		"awardDamage": 0,
		"captureZone": 0,
		"damageZone": 0,
		"missileEvades": 0,
		"shellIntgerceptions": 0,
		"score": 462,
		"team": 1,
		"name": "Сын_Буланниковa",
		"userId": "164453010"
		"squadId": 4097,
		"autoSquad": false,
		"clanTag": "[CBRU]",
	}
*/

func (tab ResultsTab) Run() {

	if imgui.BeginTabBar("results view type") {
		if imgui.BeginTabItem("ingame") {
			tab.RunIngame()
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("json") {
			imgui.InputTextMultiline("##basictext", &tab.rawJSON, imgui.ContentRegionAvail(), imgui.InputTextFlagsReadOnly, imui.ImEmptyInputCallback)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("spew") {
			imgui.InputTextMultiline("##basictext", &tab.rawSPEW, imgui.ContentRegionAvail(), imgui.InputTextFlagsReadOnly, imui.ImEmptyInputCallback)
			imgui.EndTabItem()
		}
		imgui.EndTabBar()
	}
}

func (tab ResultsTab) RunIngame() {
	teams := [][]map[string]any{{}, {}}
	playersAny, ok := tab.raw["player"].([]any)
	if !ok {
		imgui.TextUnformatted("players not array")
		return
	}
	for i, playerAny := range playersAny {
		player, ok := playerAny.(map[string]any)
		if !ok {
			imgui.TextUnformatted(fmt.Sprintf("player %d is not an object", i))
			return
		}
		team, ok := player["team"].(int64)
		if !ok {
			imgui.TextUnformatted(fmt.Sprintf("player %d no team", i))
			return
		}
		team--
		if team < 0 || team >= 2 {
			imgui.TextUnformatted(fmt.Sprintf("player %d wrong team %d", i, team))
			return
		}
		teams[team] = append(teams[team], player)
	}
	if imgui.BeginTable("resultsTable", 18) {
		imgui.TableSetupColumn("deaths")
		imgui.TableSetupColumn("caps")
		imgui.TableSetupColumn("assists")
		imgui.TableSetupColumn("ground kills")
		imgui.TableSetupColumn("air kills")
		imgui.TableSetupColumn("score")
		imgui.TableSetupColumn("name")
		imgui.TableSetupColumn("clan")
		imgui.TableSetupColumn("squad")

		imgui.TableSetupColumn("squad")
		imgui.TableSetupColumn("clan")
		imgui.TableSetupColumn("name")
		imgui.TableSetupColumn("score")
		imgui.TableSetupColumn("air kills")
		imgui.TableSetupColumn("ground kills")
		imgui.TableSetupColumn("assists")
		imgui.TableSetupColumn("caps")
		imgui.TableSetupColumn("deaths")

		imgui.TableHeadersRow()

		for r := range max(len(teams[0]), len(teams[1])) {
			imgui.TableNextRow()
			imgui.PushIDInt(int32(r * 2))
			if r < len(teams[0]) {
				p := teams[0][r]
				imgui.TableSetColumnIndex(0)
				imgui.TextUnformatted(fmt.Sprint(p["deaths"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["captureZone"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["assists"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["groundKills"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["kills"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["score"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["name"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["clanTag"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["squadId"]))
			}
			imgui.PopID()
			imgui.PushIDInt(int32(r*2 + 1))
			if r < len(teams[1]) {
				p := teams[1][r]
				imgui.TableSetColumnIndex(9)
				imgui.TextUnformatted(fmt.Sprint(p["squadId"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["clanTag"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["name"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["score"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["kills"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["groundKills"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["assists"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["captureZone"]))
				imgui.TableNextColumn()
				imgui.TextUnformatted(fmt.Sprint(p["deaths"]))
			}
			imgui.PopID()
		}

		imgui.EndTable()
	}
}
