FROM gcr.io/jenkinsxio/jx-boot:latest

ARG BUILD_DATE
ARG VERSION
ARG REVISION
ARG TARGETARCH
ARG TARGETOS

LABEL maintainer="jenkins-x"

RUN echo using jx-promote version ${VERSION} and OS ${TARGETOS} arch ${TARGETARCH} && \
  mkdir -p /home/.jx3 && \
  curl -L https://github.com/jenkins-x/jx-promote/releases/download/v${VERSION}/jx-promote-${TARGETOS}-${TARGETARCH}.tar.gz | tar xzv && \
  mv jx-promote /usr/bin

