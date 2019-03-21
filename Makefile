all: test


test:
	go test -race -v ./...

linux_plugin:
	GOOS=linux go build -v -o filebeat_throttle_linux.so -buildmode=plugin github.com/ozonru/filebeat-throttle-plugin/register/plugin

darwin_plugin:
	GOOS=darwin go build -v -o filebeat_throttle_darwin.so -buildmode=plugin github.com/ozonru/filebeat-throttle-plugin/register/plugin
