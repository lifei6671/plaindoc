.PHONY: web-dev web-build server-dev

web-dev:
	npm run web:dev

web-build:
	npm run web:build

server-dev:
	cd apps/server && go run ./cmd/server
