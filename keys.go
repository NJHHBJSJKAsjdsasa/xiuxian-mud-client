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
	server   string
	name     string
	pass     string
	register bool
	loading  bool
	err      string
	focus    int
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
	chatCh       string
	chatTo       string
	chatText     string
	typing       bool
	selectedExit int
	toasts       []toast
	fatal        string
}

func newUIState(server string) uiState {
	return uiState{
		view: viewLogin,
		page: pageGame,
		login: loginForm{
			server: server,
			focus:  1, // 服务器地址通常不用改，光标直接落在道号上
		},
		chatCh: "world",
	}
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

// chatInputHint 输入行提示：`世界> 内容▌`。
func (u *uiState) chatInputHint() string {
	return " " + chatChannelLabel(u.chatCh) + "> " + u.chatText + "▌"
}

func (u *uiState) cycleChannel() {
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
}

func (u *uiState) appendRune(r rune) {
	if u.view == viewLogin {
		switch u.login.focus {
		case 0:
			u.login.server += string(r)
		case 1:
			u.login.name += string(r)
		default:
			u.login.pass += string(r)
		}
		return
	}
	if u.view == viewCreate {
		u.createName += string(r)
		return
	}
	if u.typing {
		u.chatText += string(r)
	}
}

func (u *uiState) backspace() {
	cut := func(s string) string {
		r := []rune(s)
		if len(r) == 0 {
			return s
		}
		return string(r[:len(r)-1])
	}
	if u.view == viewLogin {
		switch u.login.focus {
		case 0:
			u.login.server = cut(u.login.server)
		case 1:
			u.login.name = cut(u.login.name)
		default:
			u.login.pass = cut(u.login.pass)
		}
		return
	}
	if u.view == viewCreate {
		u.createName = cut(u.createName)
		return
	}
	if u.typing {
		u.chatText = cut(u.chatText)
	}
}

func maskPass(s string) string {
	return strings.Repeat("*", len([]rune(s)))
}
