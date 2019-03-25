all: test


test:
	go test -race -v ./...

linux_plugin:
	GOOS=linux go build -v -o output/filebeat_throttle_linux.so -buildmode=plugin github.com/ozonru/filebeat-throttle-plugin/register/plugin

linux_plugin_docker:
	go mod vendor -v
	docker run --rm -it -v `pwd`/output:/output -v `pwd`:/go/src/github.com/ozonru/filebeat-throttle-plugin golang:1.10.6 go build -v -o /output/filebeat_throttle_linux.so -buildmode=plugin github.com/ozonru/filebeat-throttle-plugin/register/plugin


darwin_plugin:
	GOOS=darwin go build -v -o output/filebeat_throttle_darwin.so -buildmode=plugin github.com/ozonru/filebeat-throttle-plugin/register/plugin
