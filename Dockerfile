FROM gcr.io/jenkinsxio-labs-private/jxl-base:0.0.53

ENTRYPOINT ["jx-alpha-promote"]

COPY ./build/linux/jx-alpha-promote /usr/bin/jx-alpha-promote