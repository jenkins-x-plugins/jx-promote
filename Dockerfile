FROM centos:7

RUN yum install -y git

ENTRYPOINT ["jx-alpha-promote"]

COPY ./build/linux/jx-alpha-promote /usr/bin/jx-alpha-promote