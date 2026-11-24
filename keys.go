package main

import (
	"strings"
	"unicode/utf8"
)

// 按键解析：把终端读到的一把字节切成按键序列。
//
// 为什么自己切：终端把整个方向键转义序列放在一次 read 里（`ESC [ A` 三字节），
// 直接按字节处理会把一个方向键当成三个键。这里按最长匹配吃掉整个序列。

type keyKind int

const (
	keyRune keyKind = iota
	keySpecial
)

type key struct {
	kind keyKind
	r    rune   // keyRune 时有值
	name string // keySpecial 时有值：up/down/enter/esc/tab/f1…/pgup/home/end/backspace
	ctrl bool
}

func (k key) isRune(r rune) bool  { return k.kind == keyRune && k.r == r && !k.ctrl }
func (k key) is(name string) bool { return k.kind == keySpecial && k.name == name }

func special(name string) key { return key{kind: keySpecial, name: name} }

// 常见终端的 F1–F12 序列。
var fnSeqs = map[string]string{
	"\x1bOP": "f1", "\x1bOQ": "f2", "\x1bOR": "f3", "\x1bOS": "f4",
	"\x1b[11~": "f1", "\x1b[12~": "f2", "\x1b[13~": "f3", "\x1b[14~": "f4",
	"\x1b[15~": "f5", "\x1b[17~": "f6", "\x1b[18~": "f7", "\x1b[19~": "f8",
	"\x1b[20~": "f9", "\x1b[21~": "f10", "\x1b[23~": "f11", "\x1b[24~": "f12",
}

func parseKeys(buf []byte) []key {
	var out []key
	for i := 0; i < len(buf); {
		b := buf[i]
		if b == 0x1b {
			// 先试 F 键（长短不一，取最长匹配）。
			best := 0
			bestName := ""
			for seq, name := range fnSeqs {
				if len(seq) > best && len(buf)-i >= len(seq) && string(buf[i:i+len(seq)]) == seq {
					best, bestName = len(seq), name
				}
			}
			if best > 0 {
				out = append(out, special(bestName))
				i += best
				continue
			}
			// CSI：ESC [ X 或 ESC [ n ~
			if i+1 < len(buf) && buf[i+1] == '[' {
				if i+2 < len(buf) {
					switch buf[i+2] {
					case 'A':
						out = append(out, special("up"))
						i += 3
						continue
					case 'B':
						out = append(out, special("down"))
						i += 3
						continue
					case 'C':
						out = append(out, special("right"))
						i += 3
						continue
					case 'D':
						out = append(out, special("left"))
						i += 3
						continue
					case 'H':
						out = append(out, special("home"))
						i += 3
						continue
					case 'F':
						out = append(out, special("end"))
						i += 3
						continue
					case 'Z':
						out = append(out, special("shift+tab"))
						i += 3
						continue
					}
					// 数字参数：ESC [ 5 ~
					j := i + 2
					num := 0
					for j < len(buf) && buf[j] >= '0' && buf[j] <= '9' {
						num = num*10 + int(buf[j]-'0')
						j++
					}
					if j < len(buf) && buf[j] == '~' {
						switch num {
						case 1, 7:
							out = append(out, special("home"))
						case 3:
							out = append(out, special("delete"))
						case 4, 8:
							out = append(out, special("end"))
						case 5:
							out = append(out, special("pgup"))
						case 6:
							out = append(out, special("pgdn"))
						}
						i = j + 1
						continue
					}
					// 认不出的 CSI（如 Ctrl+方向键 ESC[1;5A）：整段吞掉，
					// 否则里面的 '1' ';' '5' 'A' 会被当成用户敲的字。
					k := i + 2
					for k < len(buf) && !(buf[k] >= 0x40 && buf[k] <= 0x7e) {
						k++
					}
					if k < len(buf) {
						out = append(out, special("esc"))
						i = k + 1
						continue
					}
				}
			}
			out = append(out, special("esc"))
			i++
			continue
		}
		switch b {
		case 0x03:
			out = append(out, key{kind: keySpecial, name: "c", ctrl: true})
			i++
		case 0x12:
			out = append(out, key{kind: keySpecial, name: "r", ctrl: true})
			i++
		case 0x0d, 0x0a:
			out = append(out, special("enter"))
			i++
		case 0x09:
			out = append(out, special("tab"))
			i++
		case 0x7f, 0x08:
			out = append(out, special("backspace"))
			i++
		case 0x00:
			i++
		default:
			r, size := utf8.DecodeRune(buf[i:])
			if r == utf8.RuneError && size == 1 {
				i++
				continue
			}
			if r == ' ' {
				out = append(out, special("space"))
			} else {
				out = append(out, key{kind: keyRune, r: r})
			}
			i += size
		}
	}
	return out
}

// ---- 视图状态 ----

type viewKind int

const (
	viewLogin viewKind = iota
	viewCreate
	viewMain
	viewFatal
)

type page int

const (
	pageGame page = iota
	pageCharacter
	pageBag
	pageSkills
	pageSocial
	pageMarket
	pageSect
	pageMap
	pageSettings
)

func (p page) label() string {
	switch p {
	case pageCharacter:
		return "角色"
	case pageBag:
		return "背包"
	case pageSkills:
		return "技能"
	case pageSocial:
		return "江湖"
	case pageMarket:
		return "坊市"
	case pageSect:
		return "宗门"
	case pageMap:
		return "舆图"
	case pageSettings:
		return "设置"
	default:
		return "游戏"
	}
}

func pageFromFn(name string) (page, bool) {
	switch name {
	case "f1":
		return pageCharacter, true
	case "f2":
		return pageBag, true
	case "f3":
		return pageSkills, true
	case "f4":
		return pageSocial, true
	case "f5":
		return pageMarket, true
	case "f6":
		return pageSect, true
	case "f7":
		return pageMap, true
	case "f8":
		return pageSettings, true
	}
	return pageGame, false
}

type loginForm struct {
	server    string
	name      string
	pass      string
	serverCur int
	nameCur   int
	passCur   int
	register  bool
	loading   bool
	err       string
	focus     int
}

// syncCursors 把三个光标都落到各自文本末尾；用于配置回填与自动登录。
func (f *loginForm) syncCursors() {
	f.serverCur = len([]rune(f.server))
	f.nameCur = len([]rune(f.name))
	f.passCur = len([]rune(f.pass))
}

// setFocus 切换登录表单焦点：新字段的光标落到末尾，避免在开头插字。
func (f *loginForm) setFocus(i int) {
	f.focus = i
	switch i {
	case 0:
		f.serverCur = len([]rune(f.server))
	case 1:
		f.nameCur = len([]rune(f.name))
	default:
		f.passCur = len([]rune(f.pass))
	}
}

type toast struct {
	text   string
	bad    bool
	until  int64
	sticky bool
}

type uiState struct {
	view         viewKind
	page         page
	login        loginForm
	createName   string
	createCur    int
	chatCh       string
	chatTo       string
	chatText     string
	chatCur      int
	typing       bool
	selectedExit int
	target       int // 战斗中选中的敌方序号（1 起，对应 battle.foes[].i）
	toasts       []toast
	fatal        string

	// 数据页列表光标：按页各存一份，来回翻页不会把选中的东西弄丢。
	sel [pageSettings + 1]int
	// 坊市一次买几件。
	shopQty int
	// 帮助浮层。
	help bool
	// q 打开的离开菜单：退出游戏 / 返回登录界面 / 取消。
	quitMenu bool
	quitSel  int
	// 与人物交谈的弹层。
	talkPick bool
	talkSel  int
	// 舆图平移（格）：桌面版鼠标拖动，TUI 里按 0 回中。
	mapPanX int
	mapPanY int
	// 桌面窗口模式：舆图页多显示"拖动平移"的提示。
	desktop bool
	// 需要按两次才执行的动作（logout）与第一次按下的时刻。
	confirm   string
	confirmAt int64
	// 输入行的历史：↑ 翻旧、↓ 回新；草稿在离开最新端时保存。
	inputHist   []string
	inputAt     int
	histDraft   string
	histDraftAt int
}

func newUIState(server string) uiState {
	ui := uiState{
		view: viewLogin,
		page: pageGame,
		login: loginForm{
			server: server,
			focus:  1, // 服务器地址通常不用改，光标直接落在道号上
		},
		chatCh:  "world",
		target:  1,
		shopQty: 1,
	}
	ui.login.syncCursors()
	return ui
}

// selOf / setSel 读写某页的列表光标。
func (u *uiState) selOf(p page) int {
	if int(p) < 0 || int(p) >= len(u.sel) {
		return 0
	}
	return u.sel[p]
}

func (u *uiState) setSel(p page, v int) {
	if int(p) < 0 || int(p) >= len(u.sel) {
		return
	}
	if v < 0 {
		v = 0
	}
	u.sel[p] = v
}

// rememberInput 记一条输入历史：连续重复的不记，最多 20 条。
func (u *uiState) rememberInput(text string) {
	if text == "" {
		return
	}
	if n := len(u.inputHist); n > 0 && u.inputHist[n-1] == text {
		u.inputAt = n
		return
	}
	u.inputHist = append(u.inputHist, text)
	if len(u.inputHist) > 20 {
		u.inputHist = u.inputHist[len(u.inputHist)-20:]
	}
	u.inputAt = len(u.inputHist)
}

// histStep 在输入历史里上/下翻：d=-1 更旧，d=+1 更新。
// 离开最新端时把当前草稿存起来，翻回末尾再还原，打一半的命令不会被历史顶掉。
func (u *uiState) histStep(d int) {
	if len(u.inputHist) == 0 {
		return
	}
	if u.inputAt == len(u.inputHist) && d < 0 {
		u.histDraft, u.histDraftAt = u.chatText, u.chatCur
	}
	i := u.inputAt + d
	if i < 0 {
		i = 0
	}
	if i > len(u.inputHist) {
		i = len(u.inputHist)
	}
	u.inputAt = i
	if i == len(u.inputHist) {
		u.chatText = u.histDraft
		u.chatCur = clampPos(u.histDraftAt, 0, len([]rune(u.chatText)))
		return
	}
	u.chatText = u.inputHist[i]
	u.chatCur = len([]rune(u.chatText))
}

func (u *uiState) toast(text string, bad bool) {
	if n := len(u.toasts); n > 0 {
		last := &u.toasts[n-1]
		if last.bad == bad && last.text == text {
			last.until = nowMs() + 3000
			return
		}
	}
	ttl := int64(3000)
	if bad {
		ttl = 8000
	}
	u.toasts = append(u.toasts, toast{text: text, bad: bad, until: nowMs() + ttl, sticky: bad})
	if len(u.toasts) > 3 {
		u.toasts = u.toasts[1:]
	}
}

func (u *uiState) pruneToasts() {
	now := nowMs()
	keep := u.toasts[:0]
	for _, t := range u.toasts {
		if t.sticky || t.until > now {
			keep = append(keep, t)
		}
	}
	u.toasts = keep
}

func (u *uiState) activeToast() (toast, bool) {
	if len(u.toasts) == 0 {
		return toast{}, false
	}
	return u.toasts[len(u.toasts)-1], true
}

// chatChannelLabel 频道英文名 → 中文。
func chatChannelLabel(ch string) string {
	switch ch {
	case "world":
		return "世界"
	case "region":
		return "区域"
	case "party":
		return "队伍"
	case "dm":
		return "私语"
	default:
		return ch
	}
}

// chatPrompt 输入行前缀：`世界> ` / `私语 @沈砚> `。
func (u *uiState) chatPrompt() string {
	if u.chatCh == "dm" {
		to := u.chatTo
		if to == "" {
			to = "?"
		}
		return "私语 @" + to + "> "
	}
	return chatChannelLabel(u.chatCh) + "> "
}

// cycleChannel 轮到下一个频道。没指定私语对象时不进 dm——
// 进去也只能发个空 to，服务端只会回 E-BADREQ。
func (u *uiState) cycleChannel() {
	for i := 0; i < 4; i++ {
		switch u.chatCh {
		case "world":
			u.chatCh = "region"
		case "region":
			u.chatCh = "party"
		case "party":
			u.chatCh = "dm"
		default:
			u.chatCh = "world"
		}
		if u.chatCh != "dm" || u.chatTo != "" {
			return
		}
	}
}

func clampPos(p, lo, hi int) int {
	if p < lo {
		return lo
	}
	if p > hi {
		return hi
	}
	return p
}

// activeField 返回当前正在编辑的文本与光标位置（按 rune 计）。
func (u *uiState) activeField() (*string, *int, bool) {
	if u.view == viewLogin {
		switch u.login.focus {
		case 0:
			return &u.login.server, &u.login.serverCur, true
		case 1:
			return &u.login.name, &u.login.nameCur, true
		default:
			return &u.login.pass, &u.login.passCur, true
		}
	}
	if u.view == viewCreate {
		return &u.createName, &u.createCur, true
	}
	if u.typing {
		return &u.chatText, &u.chatCur, true
	}
	return nil, nil, false
}

func (u *uiState) editInsert(r rune) {
	s, cur, ok := u.activeField()
	if !ok {
		return
	}
	rs := []rune(*s)
	pos := clampPos(*cur, 0, len(rs))
	out := make([]rune, 0, len(rs)+1)
	out = append(out, rs[:pos]...)
	out = append(out, r)
	out = append(out, rs[pos:]...)
	*s = string(out)
	*cur = pos + 1
}

func (u *uiState) editBackspace() {
	s, cur, ok := u.activeField()
	if !ok {
		return
	}
	rs := []rune(*s)
	pos := clampPos(*cur, 0, len(rs))
	if pos == 0 {
		return
	}
	*s = string(append(rs[:pos-1], rs[pos:]...))
	*cur = pos - 1
}

func (u *uiState) editDelete() {
	s, cur, ok := u.activeField()
	if !ok {
		return
	}
	rs := []rune(*s)
	pos := clampPos(*cur, 0, len(rs))
	if pos >= len(rs) {
		return
	}
	*s = string(append(rs[:pos], rs[pos+1:]...))
}

func (u *uiState) editMove(d int) {
	s, cur, ok := u.activeField()
	if !ok {
		return
	}
	*cur = clampPos(*cur+d, 0, len([]rune(*s)))
}

func (u *uiState) editHome() {
	if _, cur, ok := u.activeField(); ok {
		*cur = 0
	}
}

func (u *uiState) editEnd() {
	s, cur, ok := u.activeField()
	if !ok {
		return
	}
	*cur = len([]rune(*s))
}

func (u *uiState) appendRune(r rune) { u.editInsert(r) }

func (u *uiState) backspace() { u.editBackspace() }

func maskPass(s string) string {
	return strings.Repeat("*", len([]rune(s)))
}
