.PHONY: mocks

mocks:
	rm mocks/*
	go run github.com/vektra/mockery/v3@latest
