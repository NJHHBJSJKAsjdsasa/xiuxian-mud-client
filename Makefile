BIN := bin/playcli
GOFLAGS := -trimpath

.PHONY: build exe launcher dll dist test check clean

# Windows 上同时产出 .exe：双击与脚本调用都要能用。
# 改完代码就 make build，不要只跑 go test——用户要的是能直接跑的产物。
build:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "-s -w" -o $(BIN) .
	@if [ "$$(go env GOOS)" = "windows" ]; then \
		CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "-s -w" -o $(BIN).exe .; \
		echo "built $(BIN).exe"; \
		CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "-s -w -H=windowsgui" -o bin/launcher.exe ./cmd/launcher; \
		echo "built bin/launcher.exe"; fi

launcher:
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "-s -w -H=windowsgui" -o bin/launcher.exe ./cmd/launcher
	@echo "built bin/launcher.exe"

# 游戏本体编成 DLL；gamehost.dll 由系统 rundll32 加载，只负责 LoadLibrary + RunGame。
dll:
	CGO_ENABLED=1 go build $(GOFLAGS) -tags dll -buildmode=c-shared -ldflags "-s -w" -o bin/playcli.dll .
	gcc -O2 -shared -static -s -o bin/gamehost.dll packaging/gamehost_dll.c
	@echo "built bin/playcli.dll + bin/gamehost.dll"

# Windows 发布目录：启动器只是入口，游戏本体与资源、清单都作为独立文件随包。
dist: build dll
	python packaging/build_dist.py

exe: build

test:
	go test ./...

check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "gofmt: 上面这些文件需要格式化"; exit 1; }
	go vet ./...
	go test ./...
	python tools/doc-check.py

clean:
	rm -rf bin
