package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// 网络层：REST 登录 + WebSocket 长连接。
//
// 只认协议，不懂业务语义：UI 下命令（netCmd），网络层回事件（netEvent）。
// 所有写操作都发生在这一个 goroutine 里，读走独立 goroutine 只做解析与投递，
// 避免 gorilla 的连接被并发读写。

type cmdKind int

const (
	cmdLogin cmdKind = iota
	cmdAct
	cmdChat
	cmdLogout
	cmdShutdown
)

type netCmd struct {
	kind     cmdKind
	server   string
	name     string
	pass     string
	register bool
	action   string
	data     any
	ch       string
	to       string
	text     string
}

type connKind int

const (
	connIdle connKind = iota
	connLoggingIn
	connConnecting
	connOnline
	connBackoff
	connOffline
	connFatal
)

type connStatus struct {
	kind   connKind
	detail string
	secs   int
}

func (c connStatus) badge() (rgb, string) {
	switch c.kind {
	case connOnline:
		return colGood, "在线"
	case connConnecting:
		return colWarn, "连接中"
	case connLoggingIn:
		return colWarn, "登录中"
	case connBackoff:
		return colWarn, fmt.Sprintf("重连 %ds", c.secs)
	case connOffline:
		return colBad, c.detail
	case connFatal:
		return colBad, c.detail
	default:
		return colFaint, "未连接"
	}
}

// 事件：用具体类型 + type switch，比一个万能 struct 干净。
type netEvent interface{}

// evStatus 内嵌 connStatus：构造时可直接 evStatus{connStatus{...}}。
type evStatus struct{ connStatus }
type evReady struct{ r *readyView }
type evState struct{ p *playerView }
type evVital struct{ v *vitalView }
type evLogs struct{ lines []logLine }
type evMap struct{ m *mapView }
type evChat struct{ c chatMsg }
type evPresence struct{ n int64 }
type evBattle struct{ raw json.RawMessage }
type evErr struct{ code, msg string }
type evAnnounce struct{ text string }

// httpBase 把 ws:// 转成 http://，缺协议时补 http://。
func httpBase(server string) string {
	s := strings.TrimSuffix(strings.TrimSpace(server), "/")
	s = strings.TrimPrefix(s, "ws://")
	if strings.HasPrefix(strings.TrimSpace(server), "wss://") {
		s = strings.TrimPrefix(strings.TrimSpace(server), "wss://")
		s = "https://" + s
	} else if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	return s
}

// wsURL 把 http(s):// 转成 ws(s):// 并补 /ws。
func wsURL(server string) string {
	s := strings.TrimSuffix(strings.TrimSpace(server), "/")
	switch {
	case strings.HasPrefix(s, "http://"):
		s = "ws://" + strings.TrimPrefix(s, "http://")
	case strings.HasPrefix(s, "https://"):
		s = "wss://" + strings.TrimPrefix(s, "https://")
	case !strings.HasPrefix(s, "ws://") && !strings.HasPrefix(s, "wss://"):
		s = "ws://" + s
	}
	if !strings.HasSuffix(s, "/ws") {
		s += "/ws"
	}
	return s
}

var httpClient = &http.Client{Timeout: 8 * time.Second}

type authResult struct {
	Token   string `json:"token"`
	HasChar bool   `json:"hasChar"`
	Expires int64  `json:"expiresIn"`
}

func postJSON(url string, body any, bearer string) ([]byte, int, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		}
		if json.Unmarshal(text, &e) == nil && e.Msg != "" {
			return nil, resp.StatusCode, errors.New(e.Msg)
		}
		return nil, resp.StatusCode, fmt.Errorf("服务端返回 %s", resp.Status)
	}
	return text, resp.StatusCode, nil
}

func login(server, name, pass string, register bool) (authResult, error) {
	path := "/api/login"
	if register {
		path = "/api/register"
	}
	text, _, err := postJSON(httpBase(server)+path, map[string]string{"name": name, "pass": pass}, "")
	if err != nil {
		return authResult{}, err
	}
	var out authResult
	if err := json.Unmarshal(text, &out); err != nil {
		return authResult{}, fmt.Errorf("登录响应无法解析：%v", err)
	}
	if out.Token == "" {
		return authResult{}, errors.New("服务端没有返回 token")
	}
	return out, nil
}

func logout(server, token string) {
	_, _, _ = postJSON(httpBase(server)+"/api/logout", map[string]any{}, token)
}

type sessionEnd struct {
	kind   int
	code   int
	reason string
}

const (
	endShutdown = iota
	endLogout
	endFatal
	endClosed
)

// runNet 是网络主循环：等登录 → 建连 → 会话 → 断线退避 → 重连，直到退出。
func runNet(cmds <-chan netCmd, evs chan<- netEvent) {
	send := func(e netEvent) {
		select {
		case evs <- e:
		default: // 满了就丢：界面每 250ms 重绘，丢一条不会卡住网络
		}
	}

	for {
		var first netCmd
		// 等登录命令；其它命令在未登录时没有意义。
		for {
			c, ok := <-cmds
			if !ok {
				return
			}
			if c.kind == cmdLogin {
				first = c
				break
			}
			if c.kind == cmdShutdown {
				return
			}
		}
		send(evStatus{connStatus{kind: connLoggingIn}})

		auth, err := login(first.server, first.name, first.pass, first.register)
		if err != nil {
			send(evErr{code: errAuth, msg: err.Error()})
			send(evStatus{connStatus{kind: connIdle}})
			continue
		}

		attempt := 0
		for {
			send(evStatus{connStatus{kind: connConnecting}})
			end := wsSession(first.server, auth.Token, cmds, send)
			switch end.kind {
			case endShutdown:
				return
			case endLogout:
				logout(first.server, auth.Token)
				send(evStatus{connStatus{kind: connOffline, detail: "已退出登录"}})
			case endFatal:
				send(evStatus{connStatus{kind: connFatal, detail: end.reason}})
				return
			case endClosed:
				ck := classifyClose(end.code)
				if !ck.retryable() {
					msg := ck.message()
					if end.reason != "" {
						msg = fmt.Sprintf("%s（%d %s）", msg, end.code, end.reason)
					}
					send(evStatus{connStatus{kind: connOffline, detail: msg}})
					break
				}
				attempt++
				secs := ck.backoffSecs(attempt)
				send(evStatus{connStatus{kind: connBackoff, secs: secs, detail: end.reason}})
				if !waitBackoff(cmds, secs) {
					return
				}
				continue
			}
			break
		}
	}
}

// waitBackoff 退避等待，期间可被退出/登出打断。返回 false 表示整个网络任务该结束。
func waitBackoff(cmds <-chan netCmd, secs int) bool {
	deadline := time.After(time.Duration(secs) * time.Second)
	for {
		select {
		case <-deadline:
			return true
		case c, ok := <-cmds:
			if !ok {
				return false
			}
			switch c.kind {
			case cmdShutdown, cmdLogout:
				return false
			case cmdLogin:
				return false
			}
		}
	}
}

type readResult struct {
	data []byte
	err  error
	code int
}

// wsSession 跑一次连接：握手、收发、心跳，直到断开。
func wsSession(server, token string, cmds <-chan netCmd, send func(netEvent)) sessionEnd {
	dialer := websocket.Dialer{HandshakeTimeout: 8 * time.Second}
	conn, _, err := dialer.Dial(wsURL(server), nil)
	if err != nil {
		return sessionEnd{kind: endClosed, code: 1001, reason: "连接失败：" + err.Error()}
	}
	defer conn.Close()

	conn.SetReadLimit(1 << 20)
	if err := conn.WriteMessage(websocket.TextMessage, helloFrame(token)); err != nil {
		return sessionEnd{kind: endClosed, code: 1001, reason: "发送 hello 失败"}
	}

	var lastRx atomic.Int64
	lastRx.Store(nowMs())

	in := make(chan readResult, 64)
	// 会话结束时关掉 done：读协程如果正卡在投递上，会立刻退出，不留下僵尸。
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				code := 1001
				reason := err.Error()
				var ce *websocket.CloseError
				if errors.As(err, &ce) {
					code, reason = ce.Code, ce.Text
					if code == 1005 || code == 1006 {
						// 没带关闭帧的断开：按可重连处理。
						code = 1001
					}
				}
				select {
				case in <- readResult{err: err, code: code, data: []byte(reason)}:
				case <-done:
				}
				return
			}
			lastRx.Store(nowMs())
			select {
			case in <- readResult{data: data}:
			case <-done:
				return
			}
		}
	}()

	var seq int64
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	watch := time.NewTicker(5 * time.Second)
	defer watch.Stop()

	for {
		select {
		case r := <-in:
			if r.err != nil {
				return sessionEnd{kind: endClosed, code: r.code, reason: string(r.data)}
			}
			if end, stop := handleFrame(r.data, send); stop {
				return end
			}
		case <-ping.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, pingFrame()); err != nil {
				return sessionEnd{kind: endClosed, code: 1001, reason: "心跳发送失败"}
			}
		case <-watch.C:
			if nowMs()-lastRx.Load() > 60_000 {
				return sessionEnd{kind: endClosed, code: 1001, reason: "心跳超时"}
			}
		case c, ok := <-cmds:
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return sessionEnd{kind: endShutdown}
			}
			switch c.kind {
			case cmdShutdown:
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return sessionEnd{kind: endShutdown}
			case cmdLogout:
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				_ = conn.WriteMessage(websocket.TextMessage, closeFrame())
				_ = conn.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return sessionEnd{kind: endLogout}
			case cmdAct:
				seq++
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, actFrame(seq, c.action, c.data)); err != nil {
					return sessionEnd{kind: endClosed, code: 1001, reason: "发送失败"}
				}
			case cmdChat:
				seq++
				d := map[string]any{"ch": c.ch, "text": c.text}
				if c.to != "" {
					d["to"] = c.to
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, actFrame(seq, actChat, d)); err != nil {
					return sessionEnd{kind: endClosed, code: 1001, reason: "发送失败"}
				}
			case cmdLogin:
				// 已在线又发登录：界面只在离线时给入口，这里忽略。
			}
		}
	}
}

// handleFrame 解析一帧并投递事件。返回 stop=true 表示会话该结束了。
func handleFrame(data []byte, send func(netEvent)) (sessionEnd, bool) {
	var f inFrame
	if err := json.Unmarshal(data, &f); err != nil {
		return sessionEnd{}, false
	}
	switch f.T {
	case "ready":
		var r readyView
		if err := json.Unmarshal(f.D, &r); err != nil {
			return sessionEnd{kind: endFatal, reason: "ready 帧字段不兼容：" + err.Error()}, true
		}
		send(evReady{r: &r})
		send(evStatus{connStatus{kind: connOnline}})
	case "state":
		var p playerView
		if json.Unmarshal(f.D, &p) == nil {
			send(evState{p: &p})
		}
	case "vital":
		var v vitalView
		if json.Unmarshal(f.D, &v) == nil {
			send(evVital{v: &v})
		}
	case "log":
		var payload struct {
			Lines []logLine `json:"lines"`
		}
		if json.Unmarshal(f.D, &payload) == nil && len(payload.Lines) > 0 {
			send(evLogs{lines: payload.Lines})
		}
	case "map":
		var m mapView
		if json.Unmarshal(f.D, &m) == nil {
			send(evMap{m: &m})
		}
	case "chat":
		var c chatMsg
		if json.Unmarshal(f.D, &c) == nil {
			send(evChat{c: c})
		}
	case "presence":
		var p struct {
			Count int64 `json:"count"`
		}
		if json.Unmarshal(f.D, &p) == nil {
			send(evPresence{n: p.Count})
		}
	case "battle":
		send(evBattle{raw: f.D})
	case "err":
		var e struct {
			Code string `json:"code"`
			Msg  string `json:"msg"`
		}
		_ = json.Unmarshal(f.D, &e)
		if e.Code == "" {
			e.Code = errBadReq
		}
		if e.Msg == "" {
			e.Msg = "操作失败"
		}
		send(evErr{code: e.Code, msg: e.Msg})
	case "announce":
		var a struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(f.D, &a) == nil && a.Text != "" {
			send(evAnnounce{text: a.Text})
		}
	case "evict":
		var e struct {
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(f.D, &e)
		if e.Reason == "" {
			e.Reason = "账号在别处登录"
		}
		send(evStatus{connStatus{kind: connOffline, detail: e.Reason}})
	}
	return sessionEnd{}, false
}
