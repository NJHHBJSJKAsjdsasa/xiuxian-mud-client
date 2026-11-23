package main

import (
	"strings"
	"testing"
)

// richApp 在 testApp 的基础上补齐背包/技能/货架，专门用来测"操作"。
func richApp() *app {
	a := testApp()
	a.st.player.Bag = []bagItem{
		{ID: "itm_huiqi", N: 3, Name: "回气丹", Kind: "pill", Sell: 24},
		{ID: "itm_qingfeng", N: 1, Name: "青锋剑", Kind: "weapon", Sell: 120, Equipped: true, Slot: "weapon"},
		{ID: "itm_lingzhi", N: 7, Name: "灵芝", Kind: "herb", Sell: 8},
		{ID: "itm_budiao", N: 1, Name: "步雕靴", Kind: "boots", Sell: 90},
	}
	a.st.player.SkillBook = []skillItem{
		{ID: "skl_jianqi", Name: "剑气诀", MP: 4, Power: 1.2, Equipped: true, Slot: 0},
		{ID: "skl_tengfu", Name: "腾符", MP: 6, Power: 0.9},
		{ID: "skl_yufeng", Name: "御风", MP: 3, Power: 0.6},
	}
	a.st.player.Slots = []string{"skl_jianqi"}
	a.st.player.Stage = 5
	a.st.catalog = &catalogView{Shop: []shopItem{
		{ID: "itm_huiqi", Name: "回气丹", Kind: "pill", Price: 60, Min: 0, Sell: 24, Desc: "回复三成气血"},
		{ID: "itm_juyuan", Name: "聚元丹", Kind: "pill", Price: 70, Min: 0, Sell: 28, Desc: "回复四成真气"},
		{ID: "itm_ningshen", Name: "凝神丹", Kind: "pill", Price: 900, Min: 5, Sell: 360, Desc: "修炼速度提升六成"},
		{ID: "itm_qingfeng", Name: "青锋剑", Kind: "weapon", Price: 480, Min: 3, Sell: 192, Desc: "攻 +8"},
	}}
	return a
}

func noCmd(t *testing.T, a *app) {
	t.Helper()
	select {
	case c := <-a.cmds:
		t.Fatalf("不该发命令：%+v", c)
	default:
	}
}

func TestBagCursorNavigation(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.onKey(special("down"))
	if got := a.ui.selOf(pageBag); got != 1 {
		t.Fatalf("↓ 后应为 1，实为 %d", got)
	}
	a.onKey(special("end"))
	if got := a.ui.selOf(pageBag); got != len(a.st.player.Bag)-1 {
		t.Fatalf("End 应到末项，实为 %d", got)
	}
	a.onKey(special("down"))
	if got := a.ui.selOf(pageBag); got != len(a.st.player.Bag)-1 {
		t.Fatalf("到底后不该越过，实为 %d", got)
	}
	a.onKey(special("home"))
	if got := a.ui.selOf(pageBag); got != 0 {
		t.Fatalf("Home 应到首项，实为 %d", got)
	}
}

func TestBagEnterUsesPillAndEquipsGear(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag

	a.onKey(special("enter")) // 回气丹
	c := nextCmd(t, a)
	if c.action != actUse {
		t.Fatalf("丹药 Enter 应发 use，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["item"] != "itm_huiqi" || d["n"] != 1 {
		t.Fatalf("use 载荷不对：%+v", d)
	}

	a.ui.setSel(pageBag, 3) // 步雕靴，未装备
	a.onKey(special("enter"))
	c = nextCmd(t, a)
	if c.action != actEquip {
		t.Fatalf("装备 Enter 应发 equip，实为 %q", c.action)
	}
	d, _ = c.data.(map[string]any)
	if d["item"] != "itm_budiao" {
		t.Fatalf("equip 载荷不对：%+v", d)
	}
}

func TestBagEnterUnequipsEquipped(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.ui.setSel(pageBag, 1) // 已装备的青锋剑
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.action != actUnequip {
		t.Fatalf("已装备项 Enter 应卸下，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["slot"] != "weapon" {
		t.Fatalf("卸下槽位不对：%+v", d)
	}
}

func TestBagEquipKeyOnEquippedOnlyToasts(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.ui.setSel(pageBag, 1) // 已装备的青锋剑
	a.onKey(key{kind: keyRune, r: 'e'})
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "已经穿着") {
		t.Fatalf("对已装备项按 e 只该提示，实为 %+v", tt)
	}
}

func TestBagEquipKeyOnHerbToasts(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.ui.setSel(pageBag, 2) // 灵芝
	a.onKey(key{kind: keyRune, r: 'e'})
	noCmd(t, a)
	if _, ok := a.ui.activeToast(); !ok {
		t.Fatal("对草药按 e 应提示")
	}
}

func TestBagSellGuardsEquipped(t *testing.T) {
	a := richApp()
	a.ui.page = pageBag
	a.ui.setSel(pageBag, 1) // 已装备
	a.onKey(key{kind: keyRune, r: 's'})
	noCmd(t, a)
	if _, ok := a.ui.activeToast(); !ok {
		t.Fatal("卖已装备的物品应提示先卸下")
	}

	a.ui.setSel(pageBag, 2) // 灵芝
	a.onKey(key{kind: keyRune, r: 's'})
	c := nextCmd(t, a)
	if c.action != actSell {
		t.Fatalf("应发 sell，实为 %q", c.action)
	}
}

func TestSkillsToggleSendsFullList(t *testing.T) {
	a := richApp()
	a.ui.page = pageSkills
	a.ui.setSel(pageSkills, 1) // 腾符，未装配
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.action != actSetSkills {
		t.Fatalf("应发 setSkills，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	ids, _ := d["ids"].([]string)
	if len(ids) != 2 || ids[0] != "skl_jianqi" || ids[1] != "skl_tengfu" {
		t.Fatalf("装配后的名单不对：%+v", ids)
	}

	a.ui.setSel(pageSkills, 0) // 剑气诀，已装配
	a.onKey(special("enter"))
	c = nextCmd(t, a)
	d, _ = c.data.(map[string]any)
	ids, _ = d["ids"].([]string)
	if len(ids) != 0 {
		t.Fatalf("卸下后名单应为空，实为 %+v", ids)
	}
}

func TestSkillsSlotFull(t *testing.T) {
	a := richApp()
	a.st.player.Stage = 0 // 只有 1 个槽
	a.ui.page = pageSkills
	a.ui.setSel(pageSkills, 1)
	a.onKey(special("enter"))
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "技能槽已满") {
		t.Fatalf("槽满应提示，实为 %+v", tt)
	}
}

func TestMarketBuyQuantity(t *testing.T) {
	a := richApp()
	a.ui.page = pageMarket
	a.ui.shopQty = 1
	a.ui.setSel(pageMarket, 0) // 回气丹 60

	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.action != actBuy {
		t.Fatalf("应发 buy，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["item"] != "itm_huiqi" || d["n"] != 1 {
		t.Fatalf("buy 载荷不对：%+v", d)
	}

	a.onKey(key{kind: keyRune, r: '+'})
	a.onKey(key{kind: keyRune, r: '+'})
	if a.ui.shopQty != 3 {
		t.Fatalf("数量应为 3，实为 %d", a.ui.shopQty)
	}
	a.onKey(special("enter"))
	c = nextCmd(t, a)
	d, _ = c.data.(map[string]any)
	if d["n"] != 3 {
		t.Fatalf("应按 3 件买，实为 %+v", d)
	}
}

func TestMarketRejectsUnaffordable(t *testing.T) {
	a := richApp()
	a.ui.page = pageMarket
	a.ui.setSel(pageMarket, 2) // 凝神丹 900，身上 180
	a.onKey(special("enter"))
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "灵石不够") {
		t.Fatalf("灵石不够应提示，实为 %+v", tt)
	}
}

func TestMarketStageGate(t *testing.T) {
	a := richApp()
	a.st.player.Stage = 0
	a.ui.page = pageMarket
	a.ui.setSel(pageMarket, 3) // 青锋剑，需 3 阶
	a.onKey(special("enter"))
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "阶") {
		t.Fatalf("境界不够应提示，实为 %+v", tt)
	}
}

func TestChatWhisperCommand(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "/w 白露 在吗"
	a.onKey(special("enter"))

	c := nextCmd(t, a)
	if c.kind != cmdChat || c.ch != "dm" || c.to != "白露" || c.text != "在吗" {
		t.Fatalf("私语载荷不对：%+v", c)
	}
	if a.ui.chatTo != "白露" {
		t.Fatalf("回复对象应记为白露，实为 %q", a.ui.chatTo)
	}
	if a.ui.typing {
		t.Fatal("送出后应退出输入态")
	}
}

func TestChatReplyCommand(t *testing.T) {
	a := richApp()
	a.ui.chatTo = "白露"
	a.ui.typing = true
	a.ui.chatText = "/r 收到了"
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.ch != "dm" || c.to != "白露" || c.text != "收到了" {
		t.Fatalf("/r 载荷不对：%+v", c)
	}
}

func TestChatSlashGoesToServerCommand(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "/任务"
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.kind != cmdText || c.text != "任务" {
		t.Fatalf("斜杠命令应走 t=cmd，实为 %+v", c)
	}
}

func TestChatVerbGoesToServerCommand(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "排行 修为"
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.kind != cmdText || c.text != "排行 修为" {
		t.Fatalf("动词开头应走 t=cmd，实为 %+v", c)
	}
}

func TestChatCanTypeSpaces(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "/mail"
	for _, r := range []rune("list") {
		a.onKey(key{kind: keyRune, r: r})
	}
	a.onKey(special("space"))
	a.onKey(key{kind: keyRune, r: 'x'})
	if a.ui.chatText != "/maillist x" {
		t.Fatalf("空格没进输入行：%q", a.ui.chatText)
	}
}

func TestChatPlainTextStaysChat(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "有人在吗"
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.kind != cmdChat || c.text != "有人在吗" {
		t.Fatalf("普通文本应发 chat，实为 %+v", c)
	}
}

func TestChatDoubleSlashForcesPlain(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "//任务"
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.kind != cmdChat || c.text != "任务" {
		t.Fatalf("// 应强制当普通发言，实为 %+v", c)
	}
}

func TestChatFullWidthSlash(t *testing.T) {
	a := richApp()
	a.ui.typing = true
	a.ui.chatText = "／w 白露 在吗"
	a.onKey(special("enter"))
	c := nextCmd(t, a)
	if c.ch != "dm" || c.to != "白露" {
		t.Fatalf("全角斜杠也应认，实为 %+v", c)
	}
}

func TestChatHistoryRecall(t *testing.T) {
	a := richApp()
	a.ui.rememberInput("第一句")
	a.ui.rememberInput("第二句")
	a.ui.typing = true

	a.onKey(special("up"))
	if a.ui.chatText != "第二句" {
		t.Fatalf("↑ 应取最近一句，实为 %q", a.ui.chatText)
	}
	a.onKey(special("up"))
	if a.ui.chatText != "第一句" {
		t.Fatalf("再 ↑ 应取更早一句，实为 %q", a.ui.chatText)
	}
	a.onKey(special("down"))
	if a.ui.chatText != "第二句" {
		t.Fatalf("↓ 应回到最近一句，实为 %q", a.ui.chatText)
	}
	a.onKey(special("down"))
	if a.ui.chatText != "" {
		t.Fatalf("翻到底应为空行，实为 %q", a.ui.chatText)
	}
}

func TestQuitNeedsTwoPresses(t *testing.T) {
	a := richApp()
	a.onKey(key{kind: keyRune, r: 'q'})
	if a.quit {
		t.Fatal("第一次按 q 不该退出")
	}
	if _, ok := a.ui.activeToast(); !ok {
		t.Fatal("第一次按 q 应提示再按一次")
	}
	a.onKey(key{kind: keyRune, r: 'q'})
	if !a.quit {
		t.Fatal("第二次按 q 应退出")
	}
}

func TestLogoutNeedsTwoPresses(t *testing.T) {
	a := richApp()
	a.onKey(key{kind: keyRune, r: 'o'})
	if a.ui.view != viewMain {
		t.Fatal("第一次按 o 不该退出登录")
	}
	if _, ok := a.ui.activeToast(); !ok {
		t.Fatal("第一次按 o 应提示再按一次")
	}
	a.onKey(key{kind: keyRune, r: 'o'})
	if a.ui.view != viewLogin {
		t.Fatalf("第二次按 o 应回登录页，实为 %v", a.ui.view)
	}
}

func TestQuitConfirmCancelledByOtherKey(t *testing.T) {
	a := richApp()
	a.onKey(key{kind: keyRune, r: 'q'})
	a.onKey(key{kind: keyRune, r: 'c'}) // 打坐会发命令，同时也该撤销确认
	a.onKey(key{kind: keyRune, r: 'q'})
	if a.quit {
		t.Fatal("中间按了别的键，再按 q 应重新计数")
	}
}

func TestLogScrollKeepsPosition(t *testing.T) {
	a := richApp()
	a.logBack = 5
	a.handleEvent(evLogs{lines: []logLine{{K: "sys", S: "一"}, {K: "sys", S: "二"}}})
	if a.logBack != 7 {
		t.Fatalf("新到两行后离底距离应为 7，实为 %d", a.logBack)
	}
	a.logBack = 0
	a.handleEvent(evLogs{lines: []logLine{{K: "sys", S: "三"}}})
	if a.logBack != 0 {
		t.Fatalf("贴底时新日志不该改变 logBack，实为 %d", a.logBack)
	}
}

func TestBlockedExitExplainsWhy(t *testing.T) {
	a := richApp()
	a.st.world.Exits[2].Cond = strp("fly")
	a.ui.selectedExit = 2
	a.onKey(special("enter"))
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "御空") {
		t.Fatalf("走不通应说清楚原因，实为 %+v", tt)
	}
}

func TestEnterWhileTravelingToasts(t *testing.T) {
	a := richApp()
	a.st.player.Travel = &travelView{To: "b", Name: "青云·浅滩", ArriveAt: a.st.serverNow() + 5000}
	a.onKey(special("enter"))
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "赶路") {
		t.Fatalf("赶路中按 Enter 应提示，实为 %+v", tt)
	}
}

func TestHelpToggle(t *testing.T) {
	a := richApp()
	a.onKey(key{kind: keyRune, r: '?'})
	if !a.ui.help {
		t.Fatal("? 应打开帮助")
	}
	// 帮助开着时按 q 只收帮助，不退出。
	a.onKey(key{kind: keyRune, r: 'q'})
	if a.ui.help {
		t.Fatal("q 应收起帮助")
	}
	if a.quit {
		t.Fatal("收起帮助不该退出程序")
	}
}

func TestPageCycle(t *testing.T) {
	a := richApp()
	a.onKey(key{kind: keyRune, r: ']'})
	if a.ui.page != pageCharacter {
		t.Fatalf("] 应进第一页，实为 %v", a.ui.page)
	}
	a.onKey(key{kind: keyRune, r: '['})
	if a.ui.page != pageSettings {
		t.Fatalf("从第一页按 [ 应到最后一页，实为 %v", a.ui.page)
	}
}

func TestBattleCycleFoe(t *testing.T) {
	a := battleApp()
	a.onKey(special("down"))
	if a.ui.target != 2 {
		t.Fatalf("↓ 应选第二只，实为 %d", a.ui.target)
	}
	a.onKey(special("down"))
	if a.ui.target != 1 {
		t.Fatalf("再 ↓ 应回到第一只，实为 %d", a.ui.target)
	}
	a.st.battle.Foes[1].Dead = true
	a.onKey(special("down"))
	if a.ui.target != 1 {
		t.Fatalf("倒地的怪应跳过，实为 %d", a.ui.target)
	}
}

func TestBattleWaitsForTurn(t *testing.T) {
	a := battleApp()
	a.st.battle.State = "enemy"
	a.onKey(key{kind: keyRune, r: 'a'})
	noCmd(t, a)
	tt, ok := a.ui.activeToast()
	if !ok || !strings.Contains(tt.text, "等对方") {
		t.Fatalf("不到自己出手应提示，实为 %+v", tt)
	}
}

func TestBattleUsePill(t *testing.T) {
	a := battleApp()
	a.st.player.Bag = []bagItem{{ID: "itm_huiqi", N: 2, Name: "回气丹", Kind: "pill"}}
	a.onKey(key{kind: keyRune, r: 'i'})
	c := nextCmd(t, a)
	if c.action != actBattle {
		t.Fatalf("应发 battle，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["op"] != "item" || d["id"] != "itm_huiqi" {
		t.Fatalf("用丹载荷不对：%+v", d)
	}
}

func TestFullWidthDigitsInBattleAndEncounter(t *testing.T) {
	a := battleApp()
	a.onKey(key{kind: keyRune, r: '２'})
	if a.ui.target != 2 {
		t.Fatalf("全角 2 应选中第二只，实为 %d", a.ui.target)
	}

	e := encounterApp()
	e.onKey(key{kind: keyRune, r: '３'})
	c := nextCmd(t, e)
	d, _ := c.data.(map[string]any)
	if d["key"] != "treasure" {
		t.Fatalf("全角 3 应选第三项，实为 %+v", d)
	}
}

func TestBattlePrefersHealingPill(t *testing.T) {
	a := battleApp()
	a.st.player.Bag = []bagItem{
		{ID: "itm_ningshen", N: 1, Name: "凝神丹", Kind: "pill"},
		{ID: "itm_huiqi", N: 2, Name: "回气丹", Kind: "pill"},
	}
	a.st.catalog = &catalogView{Shop: []shopItem{
		{ID: "itm_ningshen", Name: "凝神丹", Kind: "pill", Desc: "修炼速度提升六成"},
		{ID: "itm_huiqi", Name: "回气丹", Kind: "pill", Desc: "回复三成气血"},
	}}
	a.onKey(key{kind: keyRune, r: 'i'})
	c := nextCmd(t, a)
	d, _ := c.data.(map[string]any)
	if d["id"] != "itm_huiqi" {
		t.Fatalf("战斗中应优先吃回血丹，实为 %+v", d)
	}
}

func TestBattleEscDoesNotFlee(t *testing.T) {
	a := battleApp()
	a.onKey(special("esc"))
	noCmd(t, a)
}

func TestTalkPickerSendsTalk(t *testing.T) {
	a := richApp()
	a.st.world.NPCs = []npcView{
		{ID: "npc_zhanggui", Name: "掌柜", Title: "坊市"},
		{ID: "npc_yaoshi", Name: "药师", Title: "药铺"},
	}
	a.onKey(key{kind: keyRune, r: 't'})
	if !a.ui.talkPick {
		t.Fatal("t 应打开交谈弹层")
	}
	a.onKey(key{kind: keyRune, r: '2'})
	c := nextCmd(t, a)
	if c.action != actTalk {
		t.Fatalf("应发 talk，实为 %q", c.action)
	}
	d, _ := c.data.(map[string]any)
	if d["npc"] != "npc_yaoshi" {
		t.Fatalf("talk 载荷不对：%+v", d)
	}
	if a.ui.talkPick {
		t.Fatal("说完应收起弹层")
	}
}

func TestTalkPickerEscCancel(t *testing.T) {
	a := richApp()
	a.st.world.NPCs = []npcView{{ID: "npc_zhanggui", Name: "掌柜"}}
	a.onKey(key{kind: keyRune, r: 't'})
	a.onKey(special("esc"))
	if a.ui.talkPick {
		t.Fatal("Esc 应收起交谈弹层")
	}
	noCmd(t, a)
}

func TestTalkWithoutNpcToasts(t *testing.T) {
	a := richApp()
	a.st.world.NPCs = nil
	a.onKey(key{kind: keyRune, r: 't'})
	if a.ui.talkPick {
		t.Fatal("没人时不该弹空框")
	}
	if _, ok := a.ui.activeToast(); !ok {
		t.Fatal("没人时应提示")
	}
}
