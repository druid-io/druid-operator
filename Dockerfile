from golang:1.13.5 as golang

WORKDIR /druid-operator

# Copy over the gocode
RUN mkdir -p /druid-operator
COPY . /druid-operator

# Build the druid-operator binary from gocode
RUN make -f /druid-operator/Makefile build

#############################################################
# Build the final docker image copied from build/Dockerfile #
# except the COPY lines, modified to copy from above image  # 
#############################################################
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/druid-operator \
    USER_UID=1001 \
    USER_NAME=druid-operator

# install operator binary
COPY --from=golang /druid-operator/build/_output/bin/druid-operator ${OPERATOR}

COPY --from=golang /druid-operator/build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}

