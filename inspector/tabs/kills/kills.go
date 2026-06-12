package tabKills

import (
	"fmt"
	"strconv"
	"time"

	ecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	kills2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	slot2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"

	"github.com/AllenDang/cimgui-go/imgui"
)

type KillsTab struct {
	kills   *kills2.PacketKillParser
	ecs     *ecs2.EntityManager
	players *slot2.PacketSlotParser
}

func NewKillsTab(kills *kills2.PacketKillParser, ecs *ecs2.EntityManager, players *slot2.PacketSlotParser) *KillsTab {
	return &KillsTab{
		kills:   kills,
		ecs:     ecs,
		players: players,
	}
}

func (tab *KillsTab) Name() string {
	return "Kills2"
}

func (tab *KillsTab) Init() {

}

func (tab KillsTab) Run() {
	if len(tab.kills.Kills) == 0 {
		imgui.TextUnformatted(fmt.Sprintf("Kills: %d, ECS entities: %d", len(tab.kills.Kills), len(tab.ecs.Uid_lookup)))
		return
	}
	if imgui.BeginChildStr("kills content") {
		flags := imgui.TableFlagsBorders | imgui.TableFlagsResizable | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsNoHostExtendX
		if imgui.BeginTableV("kills", 11, flags, imgui.Vec2{}, 0) {
			imgui.TableSetupColumn("n")
			imgui.TableSetupColumn("seq")
			imgui.TableSetupColumn("time")
			imgui.TableSetupColumn("killer")
			imgui.TableSetupColumn("vehicle")
			imgui.TableSetupColumn("pos")
			imgui.TableSetupColumn("weapon")
			imgui.TableSetupColumn("victim")
			imgui.TableSetupColumn("vehicle")
			imgui.TableSetupColumn("pos")
			imgui.TableSetupColumn("munition")

			imgui.TableHeadersRow()
			for i, v := range tab.kills.Kills {
				imgui.TableNextRow()
				imgui.TableNextColumn()
				imgui.TextUnformatted(strconv.FormatInt(int64(i), 10))
				imgui.TableNextColumn()
				imgui.TextUnformatted(strconv.FormatInt(int64(v.Seq), 10))
				imgui.TableNextColumn()
				imgui.TextUnformatted((time.Duration(v.CurrentTime) * time.Millisecond).String())
				imgui.TableNextColumn()

				if v.ResolvedKiller == nil {
					// imgui.PushStyleColorU32(imgui.ColText, 0x66666666)
					// imgui.TextUnformatted("!!unresolved killer!!")
					// imgui.PopStyleColor()
					imgui.TableNextColumn()
					imgui.TableNextColumn()
					imgui.TableNextColumn()
				} else {
					playerid, _ := ecs2.GetObjectData[int32](&v.ResolvedKiller.Data, "unit__playerId")
					if playerid < 0 || playerid >= int32(len(tab.players.Players)) {
						imgui.TextUnformatted(fmt.Sprintf("oob unit__playerId??? %d", playerid))
					} else {
						p := tab.players.Players[playerid]
						if p == nil {
							imgui.TextUnformatted(fmt.Sprintf("nil player %d", playerid))
						} else {
							imgui.TextUnformatted(p.Uid.Name)
						}
					}
					imgui.TableNextColumn()
					name, _ := ecs2.GetObjectData[string](&v.ResolvedKiller.Data, "unit__className")
					imgui.TextUnformatted(name)
					imgui.TableNextColumn()
					if v.ResolvedKillerPosition != nil {
						imgui.TextUnformatted(fmt.Sprintf("X %.1f Y %.1f Z %.1f", v.ResolvedKillerPosition.X, v.ResolvedKillerPosition.Y, v.ResolvedKillerPosition.Z))
					}
					imgui.TableNextColumn()
				}

				imgui.TextUnformatted(fmt.Sprint(v.PlayerWeapon))
				imgui.TableNextColumn()

				if v.ResolvedVictim == nil {
					imgui.TableNextColumn()
					imgui.TableNextColumn()
					imgui.TableNextColumn()
				} else {
					playerid, _ := ecs2.GetObjectData[int32](&v.ResolvedVictim.Data, "unit__playerId")
					if playerid < 0 || playerid >= int32(len(tab.players.Players)) {
						imgui.TextUnformatted(fmt.Sprintf("oob unit__playerId??? %d", playerid))
					} else {
						p := tab.players.Players[playerid]
						if p == nil {
							imgui.TextUnformatted(fmt.Sprintf("nil player %d", playerid))
						} else {
							imgui.TextUnformatted(p.Uid.Name)
						}
					}
					imgui.TableNextColumn()
					name, _ := ecs2.GetObjectData[string](&v.ResolvedVictim.Data, "unit__className")
					imgui.TextUnformatted(name)
					imgui.TableNextColumn()
					if v.ResolvedVictimPosition != nil {
						imgui.TextUnformatted(fmt.Sprintf("X %.1f Y %.1f Z %.1f", v.ResolvedVictimPosition.X, v.ResolvedVictimPosition.Y, v.ResolvedVictimPosition.Z))
					}
					imgui.TableNextColumn()
				}

				imgui.TextUnformatted(fmt.Sprint(v.DestroyedWeapon))
			}
			imgui.EndTable()
		}
	}
	imgui.EndChild()
}
