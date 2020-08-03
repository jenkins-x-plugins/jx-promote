FROM gcr.io/jenkinsxio-labs-private/jxl-base:0.0.60

ENTRYPOINT ["jx-promote"]

COPY ./build/linux/jx-promote /usr/bin/jx-promote