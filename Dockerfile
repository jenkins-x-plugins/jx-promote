FROM centos:7

RUN yum install -y git

ENTRYPOINT ["jx-remote"]

COPY ./build/linux/jx-remote /usr/bin/jx-remote