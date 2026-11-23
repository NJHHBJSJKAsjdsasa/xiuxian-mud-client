package main

import (
	"encoding/json"
)

// 协议模型：字段以服务端 05-PROTOCOL 的 `state.d` / `map` / `vital` 为准。
// 未知字段一律忽略（服务端只增不删）；缺省字段用零值 + 指针区分"没下发"。

const protoVersion = 1

// 动作名常量：UI 里不写字符串字面量，改名时只改这里。
const (
	actCreate       = "create"
	actLook         = "look"
	actWorldMap     = "worldMap"
	actMove         = "move"
	actExplore      = "explore"
	actCultivate    = "cultivate"
	actBreakthrough = "breakthrough"
	actChat         = "chat"
)

// 错误码：只有需要特殊处置的才列出来。
const (
	errNoChar   = "E-NOCHAR"
	errBadReq   = "E-BADREQ"
	errBusy     = "E-BUSY"
	errAuth     = "E-AUTH"
	errRate     = "E-RATE"
	errInternal = "E-INTERNAL"
)

// inFrame 是入站信封。`d` 先留着原文，按 `t` 再解成具体视图。
type inFrame struct {
	T string          `json:"t"`
	N int64           `json:"n"`
	A string          `json:"a"`
	D json.RawMessage `json:"d"`
}

func helloFrame(token string) []byte {
	b, _ := json.Marshal(map[string]any{
		"t": "hello",
		"d": map[string]any{"token": token, "proto": protoVersion},
	})
	return b
}

func actFrame(seq int64, action string, data any) []byte {
	if data == nil {
		data = map[string]any{}
	}
	b, _ := json.Marshal(map[string]any{"t": "act", "n": seq, "a": action, "d": data})
	return b
}

func pingFrame() []byte  { return []byte(`{"t":"ping"}`) }
func closeFrame() []byte { return []byte(`{"t":"close"}`) }

type playerView struct {
	Name         string      `json:"name"`
	Stage        int         `json:"stage"`
	Realm        string      `json:"realm"`
	Exp          int64       `json:"exp"`
	ExpNeed      int64       `json:"expNeed"`
	HP           int64       `json:"hp"`
	MaxHP        int64       `json:"maxHp"`
	MP           int64       `json:"mp"`
	MaxMP        int64       `json:"maxMp"`
	Atk          int64       `json:"atk"`
	Def          int64       `json:"def"`
	Spd          int64       `json:"spd"`
	Crit         float64     `json:"crit"`
	Power        int64       `json:"power"`
	Stones       int64       `json:"stones"`
	Stamina      int64       `json:"stamina"`
	StaminaMax   int64       `json:"staminaMax"`
	Toxin        int64       `json:"toxin"`
	ToxinNote    string      `json:"toxinNote"`
	FactionName  string      `json:"factionName"`
	Contrib      int64       `json:"contrib"`
	Fame         int64       `json:"fame"`
	FameTier     string      `json:"fameTier"`
	Merit        int64       `json:"merit"`
	Lives        int         `json:"lives"`
	GuildName    string      `json:"guildName"`
	GuildRank    string      `json:"guildRank"`
	GuildContrib int64       `json:"guildContrib"`
	Cultivating  bool        `json:"cultivating"`
	Region       string      `json:"region"`
	RegionName   string      `json:"regionName"`
	Loc          string      `json:"loc"`
	LocName      string      `json:"locName"`
	Travel       *travelView `json:"travel"`
	Root         *rootView   `json:"root"`
	Attrs        attrsView   `json:"attrs"`
	Free         int         `json:"free"`
	MainName     string      `json:"mainName"`
	Manuals      []string    `json:"manuals"`
	Slots        []string    `json:"slots"`
	Buffs        []buffView  `json:"buffs"`
	Bag          []bagItem   `json:"bag"`
	SkillBook    []skillItem `json:"skillBook"`
	SectID       string      `json:"sectId"`
}

type travelView struct {
	To       string `json:"to"`
	Name     string `json:"name"`
	ArriveAt int64  `json:"arriveAt"`
}

type rootView struct {
	Kind    string  `json:"kind"`
	Element string  `json:"element"`
	Name    string  `json:"name"`
	Speed   float64 `json:"speed"`
}

type attrsView struct {
	Ti   int `json:"ti"`
	Ling int `json:"ling"`
	Feng int `json:"feng"`
	Hu   int `json:"hu"`
	Shen int `json:"shen"`
}

type buffView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Until int64  `json:"until"`
}

type bagItem struct {
	ID       string `json:"id"`
	N        int64  `json:"n"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Sell     int64  `json:"sell"`
	Equipped bool   `json:"equipped"`
	Slot     string `json:"slot"`
}

type skillItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	MP       int64   `json:"mp"`
	Power    float64 `json:"power"`
	Equipped bool    `json:"equipped"`
	Slot     int     `json:"slot"`
}

type vitalView struct {
	HP          int64      `json:"hp"`
	MP          int64      `json:"mp"`
	Exp         int64      `json:"exp"`
	ExpNeed     int64      `json:"expNeed"`
	Stamina     int64      `json:"stamina"`
	Cultivating bool       `json:"cultivating"`
	Buffs       []buffView `json:"buffs"`
}

type logLine struct {
	K string `json:"k"`
	S string `json:"s"`
}

type chatMsg struct {
	Channel string `json:"channel"`
	From    string `json:"from"`
	Text    string `json:"text"`
	Ts      int64  `json:"ts"`
	NPC     bool   `json:"npc"`
}

type mapView struct {
	Loc        string     `json:"loc"`
	Name       string     `json:"name"`
	Desc       string     `json:"desc"`
	Region     string     `json:"region"`
	RegionName string     `json:"regionName"`
	Terrain    string     `json:"terrain"`
	X          *int       `json:"x"`
	Y          *int       `json:"y"`
	Path       []string   `json:"path"`
	Explored   int        `json:"explored"`
	Crowd      string     `json:"crowd"`
	Weather    *string    `json:"weather"`
	Next       *string    `json:"next"`
	Season     *string    `json:"season"`
	Daypart    *string    `json:"daypart"`
	Shard      *int64     `json:"shard"`
	Exits      []exitView `json:"exits"`
	NPCs       []npcView  `json:"npcs"`
}

type exitView struct {
	To      string  `json:"to"`
	Name    *string `json:"name"`
	Terrain string  `json:"terrain"`
	CostSec int64   `json:"costSec"`
	X       *int    `json:"x"`
	Y       *int    `json:"y"`
	Cond    *string `json:"cond"`
	OK      bool    `json:"ok"`
	Crowd   *string `json:"crowd"`
	Blocked *bool   `json:"blocked"`
	Until   *int64  `json:"until"`
	Fee     *int64  `json:"fee"`
}

type npcView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

type readyView struct {
	Proto     int         `json:"proto"`
	MinProto  int         `json:"minProto"`
	HasChar   *bool       `json:"hasChar"`
	Player    *playerView `json:"player"`
	Presence  int64       `json:"presence"`
	ChatHist  []chatMsg   `json:"chatHist"`
	ServerNow int64       `json:"serverNow"`
	Season    string      `json:"season"`
	Daypart   string      `json:"daypart"`
	World     struct {
		Online int64 `json:"online"`
	} `json:"world"`
}

func (r *readyView) hasCharacter() bool {
	if r.HasChar != nil {
		return *r.HasChar
	}
	return r.Player != nil
}

func (r *readyView) onlineCount() int64 {
	if r.Presence > 0 {
		return r.Presence
	}
	return r.World.Online
}

// 关闭码 → 处置。与 05-PROTOCOL 的 P-CONN-03 一一对应。
type closeKind int

const (
	closeNormal closeKind = iota
	closeRetry
	closeFrameTooBig
	closeServerError
	closeEvicted
	closeTokenExpired
	closeProtoMismatch
	closeRateLimited
	closeUnknown
)

func classifyClose(code int) closeKind {
	switch code {
	case 1000:
		return closeNormal
	case 1001:
		return closeRetry
	case 1009:
		return closeFrameTooBig
	case 1011:
		return closeServerError
	case 4001:
		return closeEvicted
	case 4003:
		return closeTokenExpired
	case 4004:
		return closeProtoMismatch
	case 4029:
		return closeRateLimited
	default:
		return closeUnknown
	}
}

func (k closeKind) retryable() bool {
	return k == closeRetry || k == closeServerError || k == closeRateLimited || k == closeUnknown
}

func (k closeKind) backoffSecs(attempt int) int {
	if k == closeRateLimited {
		return 60
	}
	exp := 1 << min(attempt, 5)
	return min(exp, 30)
}

func (k closeKind) message() string {
	switch k {
	case closeNormal:
		return "连接已关闭"
	case closeFrameTooBig:
		return "发出的消息过大，已被服务端断开（客户端缺陷）"
	case closeServerError:
		return "服务端异常，稍后再试"
	case closeEvicted:
		return "账号在别处登录"
	case closeTokenExpired:
		return "登录已过期，请重新登录"
	case closeProtoMismatch:
		return "客户端版本与服务端不兼容，请升级客户端"
	case closeRateLimited:
		return "操作过频被踢下线，60 秒后可重连"
	default:
		return "连接断开"
	}
}
