CDN_WEB_DIST ?= src/web/dist
export CDN_ORIGIN CDN_WEB_DIST

.PHONY: setup api web check lint format test build cdn-urls

setup:
	pnpm --dir src/web install --frozen-lockfile
	$(MAKE) -C src/api tidy

api:
	$(MAKE) -C src/api dev

web:
	pnpm --dir src/web dev

check:
	pnpm --dir src/web check
	$(MAKE) -C src/api check

lint:
	pnpm --dir src/web lint
	$(MAKE) -C src/api lint

format:
	pnpm --dir src/web format
	$(MAKE) -C src/api format

test:
	pnpm --dir src/web test
	$(MAKE) -C src/api test

build:
	pnpm --dir src/web build
	$(MAKE) -C src/api build

cdn-urls:
	@node src/web/scripts/cdn-urls.ts "$$CDN_ORIGIN" "$$CDN_WEB_DIST"
