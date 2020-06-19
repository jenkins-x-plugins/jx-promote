FROM gcr.io/jenkinsxio-labs-private/jxl-base:0.0.52

ENTRYPOINT ["jx-alpha-promote"]

COPY ./build/linux/jx-alpha-promote /usr/bin/jx-alpha-promote