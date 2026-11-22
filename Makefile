# 編譯由 GitHub Actions 負責（見 .github/workflows/build.yml）
# 本機僅提供開發用的測試與設定檔產生指令。

.PHONY: test init run panel

test:
	go test ./...

init:
	go run ./cmd/agent -init -config config.example.json

run:
	go run ./cmd/agent -config config.json -env .env

panel:
	go run ./cmd/agent -panel -config config.json -env .env
