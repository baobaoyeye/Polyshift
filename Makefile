.PHONY: proto-go proto-py proto build run clean

proto-go:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/plugin/plugin.proto

proto-py:
	source .venv/bin/activate && python3 -m grpc_tools.protoc -Iproto/plugin \
		--python_out=sdk/python/polyshift/plugin \
		--grpc_python_out=sdk/python/polyshift/plugin \
		proto/plugin/plugin.proto

proto: proto-go proto-py

build:
	go build -o bin/core cmd/core/main.go
	go build -o bin/go-hello examples/go-hello/main.go

setup-py:
	python3 -m venv .venv
	source .venv/bin/activate && pip install -r sdk/python/requirements.txt

run:
	source .venv/bin/activate && go run cmd/core/main.go

clean:
	rm -rf bin
	rm -rf sdk/python/polyshift/plugin/*_pb2*.py
