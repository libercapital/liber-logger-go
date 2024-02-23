unit-test:
	gotestsum -- ./... -failfast -race -coverprofile ./coverage.out

watch:
	gotestsum --format=pkgname --watch -- -v -race -short ./...
