package main

import (
	"encoding/json"
	"testing"
)

// 这一组盯的是"选好的去处被无故打回第一项"。
// 触发过的事故：state 帧没有 loc，客户端以为地图与角色位置永远不一致，
// 每 600ms 补拉一次 worldMap，每次应答都把 selectedExit 清零。

func TestSameLocMapKeepsSelection(t *testing.T) {
	a := testApp()
	a.ui.selectedExit = 2

	// look 的应答：同一节点、没有出口。
	look := &mapView{Loc: a.st.world.Loc, Name: a.st.world.Name, Desc: "一条小路"}
	a.handleEvent(evMap{m: look})

	if a.ui.selectedExit != 2 {
		t.Fatalf("同一节点的 map 帧不该改选中项，实为 %d", a.ui.selectedExit)
	}
}

func TestNewLocMapResetsSelection(t *testing.T) {
	a := testApp()
	a.ui.selectedExit = 2
	moved := &mapView{Loc: "loc_other", Name: "别处", Exits: []exitView{exit("x", "某地", 3, true)}}
	a.handleEvent(evMap{m: moved})
	if a.ui.selectedExit != 0 {
		t.Fatalf("换节点后选中项应归零，实为 %d", a.ui.selectedExit)
	}
}

func TestDataPageIgnoresNavAndEnter(t *testing.T) {
	a := testApp()
	a.ui.page = pageCharacter
	a.ui.selectedExit = 1

	a.onKey(special("down"))
	if a.ui.selectedExit != 1 {
		t.Fatalf("数据页按 ↓ 不该改主界面选中项，实为 %d", a.ui.selectedExit)
	}

	a.onKey(special("enter"))
	select {
	case c := <-a.cmds:
		t.Fatalf("角色页按 Enter 不该发命令：%+v", c)
	default:
	}

	// Esc 仍然要能回主界面。
	a.onKey(special("esc"))
	if a.ui.page != pageGame {
		t.Fatal("Esc 应回到主界面")
	}
}

func TestMapPageEnterRefreshes(t *testing.T) {
	a := testApp()
	a.ui.page = pageMap
	a.onKey(special("enter"))
	select {
	case c := <-a.cmds:
		if c.action != actWorldMap {
			t.Fatalf("舆图页 Enter 应刷新 worldMap，实为 %q", c.action)
		}
	default:
		t.Fatal("舆图页 Enter 应发 worldMap")
	}
}

func TestScrollLogsStopsAtBounds(t *testing.T) {
	a := testApp()
	a.onKey(special("pgup"))
	if a.logBack == 0 {
		t.Fatal("PgUp 应向上翻")
	}
	before := a.logBack
	a.onKey(special("pgup"))
	a.onKey(special("pgup"))
	if a.logBack < before {
		t.Fatal("连续 PgUp 不该回退")
	}
	if a.logBack > a.st.logs.len()-1 {
		t.Fatalf("翻过界了：%d > %d", a.logBack, a.st.logs.len()-1)
	}
	a.onKey(special("end"))
	if a.logBack != 0 {
		t.Fatal("End 应回到最新")
	}
}

func TestRefreshStaleMapOnlyWhenLocDiffers(t *testing.T) {
	a := testApp()
	a.tick()
	select {
	case c := <-a.cmds:
		t.Fatalf("位置一致时不该补拉世界地图：%+v", c)
	default:
	}

	a.st.player.Loc = "loc_other"
	a.tick()
	select {
	case c := <-a.cmds:
		if c.action != actWorldMap {
			t.Fatalf("应发 worldMap，实为 %q", c.action)
		}
	default:
		t.Fatal("位置不一致时应补拉世界地图")
	}
}

func battleApp() *app {
	a := testApp()
	a.st.battle = &battleView{
		ID: "b1", Kind: "pve", Turn: 1, Round: 1, State: "player",
		Enemy: &foeView{I: 1, Name: "山兔精", HP: 52, MaxHP: 81},
		Me:    &vitalBase{HP: 90, MaxHP: 90, MP: 40, MaxMP: 40},
		Foes: []foeView{
			{I: 1, Name: "山兔精", HP: 52, MaxHP: 81},
			{I: 2, Name: "黑风狼", HP: 30, MaxHP: 60},
		},
		Allies:  []allyView{{Name: "白露", HP: 20, MaxHP: 60}},
		CanFlee: true,
	}
	a.ui.target = 1
	return a
}

func nextCmd(t *testing.T, a *app) netCmd {
	t.Helper()
	select {
	case c := <-a.cmds:
		return c
	default:
		t.Fatal("没有发出命令")
		return netCmd{}
	}
}

func TestBattleAttackSendsTarget(t *testing.T) {
	a := battleApp()
	a.onKey(key{kind: keyRune, r: 'a'})
	c := nextCmd(t, a)
	if c.action != actBattle {
		t.Fatalf("应发 battle，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["op"] != "attack" || d["target"] != 1 {
		t.Fatalf("载荷不对：%+v", d)
	}
}

func TestBattleTargetSwitch(t *testing.T) {
	a := battleApp()
	a.onKey(key{kind: keyRune, r: '2'})
	if a.ui.target != 2 {
		t.Fatalf("按 2 应选第二只，实为 %d", a.ui.target)
	}
	// 不存在的序号不该改目标。
	a.onKey(key{kind: keyRune, r: '7'})
	if a.ui.target != 2 {
		t.Fatalf("越界序号不该改目标，实为 %d", a.ui.target)
	}
}

func TestBattleFlee(t *testing.T) {
	a := battleApp()
	a.onKey(key{kind: keyRune, r: 'f'})
	c := nextCmd(t, a)
	d, _ := c.data.(map[string]any)
	if d["op"] != "flee" {
		t.Fatalf("载荷不对：%+v", d)
	}
}

func TestBattleSkillWithoutEquipToasts(t *testing.T) {
	a := battleApp()
	a.st.player.SkillBook = nil
	a.onKey(key{kind: keyRune, r: 's'})
	select {
	case c := <-a.cmds:
		t.Fatalf("没装配技能不该发命令：%+v", c)
	default:
	}
	if _, ok := a.ui.activeToast(); !ok {
		t.Fatal("应提示还没装配技能")
	}
}

func TestStateKeepsBattle(t *testing.T) {
	// 服务端在战斗开始后也会补一条 state（探索的同一次结算里），
	// 它不该把战斗盒清掉——否则刚进战斗就看不见敌人。
	a := battleApp()
	a.handleEvent(evState{p: &playerView{Name: "沈砚", HP: 90, MaxHP: 90}})
	if a.st.battle == nil {
		t.Fatal("state 不该清战斗盒")
	}
}

func TestBattleOverClearsBox(t *testing.T) {
	a := battleApp()
	a.handleEvent(evBattle{b: &battleView{ID: "b1", Over: true, Won: true}})
	if a.st.battle != nil {
		t.Fatal("over 帧应清掉战斗盒")
	}
}

func encounterApp() *app {
	a := testApp()
	a.st.encounter = &eventView{
		ID: "enc_1",
		Options: []eventOption{
			{Key: "exp", Text: "静心打坐，参悟一缕道意"},
			{Key: "manual", Text: "翻开石缝里的残卷"},
			{Key: "treasure", Text: "搬开压着灵光的石块"},
		},
	}
	return a
}

func TestEncounterPickSendsKey(t *testing.T) {
	a := encounterApp()
	a.onKey(key{kind: keyRune, r: '2'})
	c := nextCmd(t, a)
	if c.action != actEncounter {
		t.Fatalf("应发 encounter，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["key"] != "manual" {
		t.Fatalf("选项 key 不对：%+v", d)
	}
	if a.st.encounter != nil {
		t.Fatal("选完应收起弹层")
	}
}

func TestEncounterIgnoresOtherKeys(t *testing.T) {
	a := encounterApp()
	a.onKey(key{kind: keyRune, r: 'a'})
	select {
	case c := <-a.cmds:
		t.Fatalf("奇遇弹层里不该发命令：%+v", c)
	default:
	}
	if a.st.encounter == nil {
		t.Fatal("按非数字键不该收弹层")
	}
}

func TestStateKeepsEncounter(t *testing.T) {
	a := encounterApp()
	a.handleEvent(evState{p: &playerView{Name: "沈砚"}})
	if a.st.encounter == nil {
		t.Fatal("state 不该清掉奇遇弹层")
	}
}

func TestParseBattleFrameFromDoc(t *testing.T) {
	raw := `{"id":"b_7f3a","kind":"pve","turn":1,"state":"player",
	  "enemy":{"name":"山兔精","hp":81,"maxHp":81,"element":"tu"},
	  "me":{"hp":90,"maxHp":90,"mp":40,"maxMp":40},"canFlee":true}`
	var b battleView
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	if b.Enemy == nil || b.Enemy.Name != "山兔精" || b.Enemy.HP != 81 {
		t.Fatalf("敌方解析错：%+v", b.Enemy)
	}
	if b.Me == nil || b.Me.MaxMP != 40 || !b.CanFlee {
		t.Fatalf("我方/逃跑标记解析错：%+v", b)
	}
}
