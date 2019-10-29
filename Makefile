fmt:
	test -z $$(gofmt -l -s pkg/apis/druid/v1alpha1/druid_types.go)
	test -z $$(gofmt -l -s pkg/controller/druid)

test:
	go test -mod vendor './...'

build: fmt test
	# derived from running, operator-sdk --verbose build <image>
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GO111MODULE=on go build -mod vendor -o build/_output/bin/druid-operator github.com/druid-io/druid-operator/cmd/manager 
