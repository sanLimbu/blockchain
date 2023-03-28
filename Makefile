build:	
	go build -o bin/marvincrypto

run: build
	./bin/marvincrypto

test:
	go test -v ./..		