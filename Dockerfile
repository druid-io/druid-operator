#########################
# Build the Golang code #
#########################
from golang:1.10 as golang

# Set the work dir
WORKDIR /gocode

# Set the gopath
ENV GOPATH /gocode

# Copy over the gocode
RUN mkdir -p /gocode/src/
COPY . /gocode/src/druid-app-operator

# Setup the build directory
RUN mkdir -p /build

# Build the code
RUN make -f /gocode/src/druid-app-operator/Makefile build

################################
# Build the final docker image #
################################
FROM alpine:3.6

# TODO document
RUN adduser -D druid-app-operator

# TODO document
USER druid-app-operator

# Copy binary
COPY --from=golang /build/druid-app-operator  /usr/local/bin/druid-app-operator
