FROM gcr.io/jenkinsxio/jx-cli-base:0.0.10

ENTRYPOINT ["jx-promote"]

COPY ./build/linux/jx-promote /usr/bin/jx-promote