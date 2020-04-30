# Step one: build compliance-operator
FROM registry.access.redhat.com/ubi8/ubi-minimal as builder

RUN microdnf install golang make

WORKDIR /go/src/github.com/JAORMX/compliance-profile-operator

ENV GOFLAGS=-mod=vendor

COPY . .
RUN make manager

# Step two: containerize compliance-operator
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV OPERATOR=/usr/local/bin/compliance-profile-operator \
    USER_UID=1001 \
    USER_NAME=compliance-profile-operator

# install operator binary
COPY --from=builder /go/src/github.com/JAORMX/compliance-profile-operator/build/_output/bin/compliance-profile-operator ${OPERATOR}

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
