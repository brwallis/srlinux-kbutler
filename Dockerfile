FROM golang:alpine as builder

LABEL Author "Bruce Wallis <bwallis@nokia.com>"

ARG WORK_DIR="/usr/src/kbutler"
ARG BINARY_NAME="kbutler"
ARG REPO_USER
ENV REPO_USER=$REPO_USER
ARG REPO_PASSWORD
ENV REPO_PASSWORD=$REPO_PASSWORD
ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy


RUN apk add --no-cache --virtual build-dependencies build-base=~0.5 git
RUN echo "machine github.com login ${REPO_USER} password ${REPO_PASSWORD}" > ~/.netrc

COPY . $WORK_DIR
WORKDIR $WORK_DIR
RUN make clean && \
    make build

#RUN git config --global url."https://${REPO_USER}:${REPO_PASSWORD}@github.com".insteadOf "https://github.com"

FROM alpine:3
ARG WORK_DIR="/usr/src/kbutler"
ARG BINARY_NAME="kbutler"
RUN mkdir -p /${BINARY_NAME}/bin
RUN mkdir -p /${BINARY_NAME}/yang
COPY --from=builder ${WORK_DIR}/build/$BINARY_NAME /${BINARY_NAME}/bin/
COPY --from=builder ${WORK_DIR}/appmgr/kube.yang /${BINARY_NAME}/yang
COPY --from=builder ${WORK_DIR}/appmgr/kubemgr_config.yml /${BINARY_NAME}/
WORKDIR /

LABEL io.k8s.display-name="Nokia SR Linux Kubernetes Butler"

COPY ./images/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
