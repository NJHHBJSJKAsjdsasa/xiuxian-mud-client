package main

import (
	"strings"
	"testing"
)

func sceneFor(terrain, weather string) sceneSpec {
	return sceneSpec{terrain: terrain, weather: weather, daypart: "day", pose: poseStanding}
}

func TestSceneGridSize(t *testing.T) {
	scr := newScreen(40, 12)
	drawScene(scr, 0, 0, 40, 12, sceneFor("plain", "clear"))
	if len(scr.buf) != 40*12 {
		t.Fatalf("网格大小不对：%d", len(scr.buf))
	}
	for i, line := range scr.frame() {
		if w := textWidth(line); w != 40 {
			t.Fatalf("第 %d 行宽 %d：%q", i, w, line)
		}
	}
}

func TestSceneDrawsFigureAndGround(t *testing.T) {
	scr := newScreen(40, 12)
	drawScene(scr, 0, 0, 40, 12, sceneFor("plain", "clear"))
	flat := strings.Join(scr.frame(), "\n")
	if !strings.Contains(flat, "o") {
		t.Fatal("应画出入影的头")
	}
	if !strings.Contains(flat, "=") {
		t.Fatal("应画出地平线")
	}
	if !strings.Contains(flat, "/|\\") {
		t.Fatal("应画出人形躯干")
	}
}

func TestSceneWeatherOverlay(t *testing.T) {
	scr := newScreen(40, 12)
	drawScene(scr, 0, 0, 40, 12, sceneFor("plain", "rain"))
	if !strings.Contains(strings.Join(scr.frame(), ""), "|") {
		t.Fatal("雨天应有竖线")
	}
}

func TestSceneSittingPose(t *testing.T) {
	spec := sceneFor("plain", "clear")
	spec.pose = poseSitting
	scr := newScreen(24, 10)
	drawScene(scr, 0, 0, 24, 10, spec)
	if !strings.Contains(strings.Join(scr.frame(), ""), "===") {
		t.Fatal("打坐应画成 === 的坐姿")
	}
}

func TestSceneTinyAreaDoesNotPanic(t *testing.T) {
	scr := newScreen(10, 5)
	drawScene(scr, 0, 0, 3, 2, sceneFor("woods", "fog"))
}

func TestSceneDeterministic(t *testing.T) {
	a := newScreen(50, 14)
	b := newScreen(50, 14)
	drawScene(a, 0, 0, 50, 14, sceneFor("forest", "snow"))
	drawScene(b, 0, 0, 50, 14, sceneFor("forest", "snow"))
	if strings.Join(a.frame(), "\n") != strings.Join(b.frame(), "\n") {
		t.Fatal("同样输入应画出同样的画面（不逐帧抖动）")
	}
}
