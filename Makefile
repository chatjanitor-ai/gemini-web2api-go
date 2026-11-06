-include .env

.PHONY: dev

dev:
	@test -n "$(ADMIN_TOKEN)" || { echo "Set ADMIN_TOKEN in .env before running make dev"; exit 1; }
	@ADMIN_TOKEN="$(ADMIN_TOKEN)" go run . --port 8083
