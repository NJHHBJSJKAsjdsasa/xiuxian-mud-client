BIN := bin/playcli
GOFLAGS := -trimpath

.PHONY: build exe launcher test check clean

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
