all:
	go test ./...
	go test ./... -short -race
	go test ./... -run=NONE -bench=. -benchmem -benchtime=5s -memprofile=mem.out -cpuprofile=cpu.out
	golangci-lint run
