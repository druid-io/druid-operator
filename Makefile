test:
	go test -mod vendor './...'

build: test
	# derived from running, operator-sdk --verbose build <image>
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -mod vendor -o build/_output/bin/druid-operator github.com/druid-io/druid-operator/cmd/manager 
