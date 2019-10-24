test:
	go test './...'

build: test
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /build/druid-app-operator druid-app-operator/cmd/druid-app-operator
