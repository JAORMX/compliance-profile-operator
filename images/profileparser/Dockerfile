FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

ENV USER_UID=1001 \
    USER_NAME=compliance-profile-operator

COPY build/_output/bin/profileparser /usr/local/bin/profileparser

COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/profileparser"]

USER ${USER_UID}
