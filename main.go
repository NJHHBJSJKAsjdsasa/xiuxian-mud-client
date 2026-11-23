// playcli 是《九州问道》的玩家终端客户端（与 gmcli 相对：一个玩家视角，一个管理视角）。
//
// 纯 Go + 标准库 + golang.org/x/term，渲染是手写 ANSI：不引 TUI 框架、
// 不依赖浏览器，SSH 上去就能玩。
//
// 用法：
//
//	playcli                               # 登录页
//	playcli --server ws://127.0.0.1:8080
//	playcli --login 沈砚:secret123        # 跳过登录页
//	playcli --create 沈砚                 # 登录后自动建号
//	playcli --page map                    # 进主界面后直接开某页
//	playcli --move 0                      # 进主界面后自动走第 1 个出口
//	playcli --snapshot out.txt            # 离屏渲染一屏（排查/截图用）
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

type args struct {
	server   string
	login    string
	create   string
	page     string
	move     int
	explore  int
	keys     string
	wait     int
	hasMove  bool
	snapshot string
	color    string
	width    int
	height   int
}

func parseArgs() args {
	var a args
	flag.StringVar(&a.server, "server", "", "服务端地址，如 ws://127.0.0.1:8080")
	flag.StringVar(&a.login, "login", "", "道号:密码，跳过登录页")
	flag.StringVar(&a.create, "create", "", "登录后自动建号")
	flag.StringVar(&a.page, "page", "", "起始页：game|character|bag|skills|social|market|sect|map|settings")
	flag.IntVar(&a.move, "move", 0, "进主界面后自动走第 n 个出口（从 0 起）")
	flag.IntVar(&a.explore, "explore", 0, "进主界面后自动历练 n 次（间隔 1.5 秒，联调用）")
	flag.StringVar(&a.keys, "keys", "", "快照前模拟按键（逗号分隔）：f2,down,enter；单字符直接写，如 ? 或 /")
	flag.IntVar(&a.wait, "wait", 0, "进主界面后再等 n 秒（等赶路到达、等战斗结算）")
	flag.StringVar(&a.snapshot, "snapshot", "", "离屏渲染一屏到文件后退出")
	flag.StringVar(&a.color, "color", "", "颜色：true|256|off")
	flag.IntVar(&a.width, "w", 110, "快照宽度（列）")
	flag.IntVar(&a.height, "h", 36, "快照高度（行）")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "playcli —— 九州问道 终端客户端")
		flag.PrintDefaults()
	}
	flag.Parse()
	a.hasMove = false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "move" {
			a.hasMove = true
		}
	})
	return a
}

func main() {
	argv := parseArgs()
	cfg := loadConfig()
	if argv.server != "" {
		cfg.Server = argv.server
	}
	if argv.color != "" {
		cfg.Color = argv.color
	}
	mode := parseColorMode(cfg.Color)
	if os.Getenv("NO_COLOR") != "" {
		mode = colorOff
	}

	a := newApp(cfg)
	if argv.login != "" {
		if name, pass, ok := strings.Cut(argv.login, ":"); ok {
			a.autoLogin(name, pass)
		} else {
			fmt.Fprintln(os.Stderr, "--login 需要 道号:密码 的形式")
			os.Exit(2)
		}
	}
	if argv.create != "" {
		a.autoCreate(argv.create)
	}
	if argv.page != "" {
		a.startPage(argv.page)
	}
	if argv.hasMove {
		a.autoMove(argv.move)
	}
	if argv.explore > 0 {
		a.autoExplore(argv.explore)
	}

	if argv.snapshot != "" {
		if err := runSnapshot(a, argv, mode); err != nil {
			fmt.Fprintln(os.Stderr, "快照失败：", err)
			os.Exit(1)
		}
		return
	}
	if err := runTUI(a, mode); err != nil {
		fmt.Fprintln(os.Stderr, "无法启动界面：", err)
		os.Exit(1)
	}
}

// runTUI 进入全屏界面。管道/CI 里不要调用它。
func runTUI(a *app, mode colorMode) error {
	enableVT()
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("无法进入终端原始模式：%v", err)
	}
	defer func() { _ = term.Restore(fd, old) }()

	out := bufio.NewWriterSize(os.Stdout, 1<<16)
	_, _ = out.WriteString("\x1b[?1049h\x1b[?25l\x1b[2J")
	_ = out.Flush()
	defer func() {
		_, _ = out.WriteString("\x1b[?25h\x1b[?1049l")
		_ = out.Flush()
		a.shutdown()
	}()

	keys := make(chan []key, 8)
	go func() {
		buf := make([]byte, 128)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				keys <- parseKeys(buf[:n])
			}
			if err != nil {
				close(keys)
				return
			}
		}
	}()

	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	for !a.quit {
		w, h, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || w < 40 || h < 12 {
			w, h = 100, 30
		}
		a.drain()
		a.tick()
		a.render(w, h).render(out, mode)

		select {
		case batch, ok := <-keys:
			if !ok {
				return nil
			}
			for _, k := range batch {
				a.onKey(k)
				if a.quit {
					break
				}
			}
		case ev := <-a.evs:
			a.handleEvent(ev)
		case <-tick.C:
		}
	}
	return nil
}

// runSnapshot 离屏渲染一屏：连上服务端等状态就绪，或者直接画登录页。
// 输出纯文本行（--color=true 时带 SGR），给排查与自动截图用。
func runSnapshot(a *app, argv args, mode colorMode) error {
	w, h := argv.width, argv.height
	if w < 40 {
		w = 40
	}
	if h < 12 {
		h = 12
	}

	if argv.login != "" {
		wait := 6 * time.Second
		if argv.explore > 0 {
			wait = 12 * time.Second
		}
		deadline := time.Now().Add(wait)
		for time.Now().Before(deadline) {
			// tick 负责把待办的自动登录真正发出去，缺了它快照会一直停在登录页。
			a.tick()
			select {
			case ev := <-a.evs:
				a.handleEvent(ev)
			case <-time.After(50 * time.Millisecond):
			}
			// 战斗中/奇遇里画面已经是我们想看的那一屏，不用再等满。
			if a.st.battle != nil || a.st.encounter != nil {
				break
			}
			// 不带 --explore 时，进主界面就够；带的时候要留时间把历练跑完。
			if argv.explore == 0 && a.st.hasChar && a.st.world != nil {
				break
			}
			if a.st.conn.kind == connOffline || a.st.conn.kind == connFatal {
				break
			}
		}
		if argv.wait > 0 {
			pump(a, time.Duration(argv.wait)*time.Second)
		}
		if spec := argv.keys; spec != "" {
			// 脚本按键：每个键之间留出一次往返的时间，装备/买卖这类
			// 要等服务端回 state 才看得到结果。
			for _, k := range scriptKeys(spec) {
				a.onKey(k)
				pump(a, 250*time.Millisecond)
			}
			pump(a, 700*time.Millisecond)
			goto done
		}
		// 收尾：等地图与日志齐一点，画面更像真实一帧。
		// 已经进战斗/奇遇时只等一小会儿，免得服务端等不到出手把战斗中止。
		settleFor := 700 * time.Millisecond
		if a.st.battle != nil || a.st.encounter != nil {
			settleFor = 150 * time.Millisecond
		}
		settle := time.After(settleFor)
		for {
			select {
			case ev := <-a.evs:
				a.handleEvent(ev)
			case <-settle:
				goto done
			}
		}
	}
done:
	scr := a.render(w, h)
	var b strings.Builder
	if mode == colorOff {
		for _, line := range scr.frame() {
			b.WriteString(strings.TrimRight(line, " "))
			b.WriteByte('\n')
		}
	} else {
		buf := bufio.NewWriter(&b)
		scr.render(buf, mode)
		_ = buf.Flush()
	}
	text := strings.ReplaceAll(b.String(), "\x1b[K", "\n")
	if mode != colorOff {
		text = strings.ReplaceAll(text, "\x1b[", "")
	}
	return os.WriteFile(argv.snapshot, []byte(text), 0o644)
}

// scriptKeys 把 --keys 的写法解析成按键：up/down/…/f1–f12 走特殊键，
// 其余按单个字符处理；像 "ab" 这样的多字符会拆成两个按键。
func scriptKeys(spec string) []key {
	var out []key
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		switch part {
		case "":
			continue
		case "up", "down", "left", "right", "enter", "esc", "tab", "space",
			"pgup", "pgdn", "home", "end", "backspace":
			out = append(out, special(part))
			continue
		}
		if len(part) >= 2 && part[0] == 'f' {
			// 必须按数字比："2" <= "12" 在字符串序里是假，会把 f2 当成字符 f 和 2。
			if n, err := strconv.Atoi(part[1:]); err == nil && n >= 1 && n <= 12 {
				out = append(out, special(part))
				continue
			}
		}
		for _, r := range part {
			out = append(out, key{kind: keyRune, r: r})
		}
	}
	return out
}

// pump 在 d 时间里持续消化事件与定时任务，让脚本按键能看到服务端回包。
func pump(a *app, d time.Duration) {
	deadline := time.After(d)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case ev := <-a.evs:
			a.handleEvent(ev)
		case <-tick.C:
			a.tick()
		case <-deadline:
			return
		}
	}
}
