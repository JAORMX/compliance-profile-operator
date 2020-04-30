# Step one: build compliance-operator
FROM registry.access.redhat.com/ubi8/ubi-minimal as builder

RUN microdnf install golang make

WORKDIR /go/src/github.com/JAORMX/compliance-profile-operator

ENV GOFLAGS=-mod=vendor

COPY . .
RUN make profileparser

# Step two: containerize profileparser
FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV USER_UID=1001 \
    USER_NAME=compliance-profile-operator

COPY --from=builder /go/src/github.com/JAORMX/compliance-profile-operator/build/_output/bin/profileparser /usr/local/bin/profileparser

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/profileparser"]

USER ${USER_UID}
