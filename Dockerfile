FROM centos:centos7.9.2009

RUN curl -LO https://go.dev/dl/go1.26.1.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.26.1.linux-amd64.tar.gz && \
    mkdir /app

ENV PATH=$PATH:/usr/local/go/bin
WORKDIR /app
