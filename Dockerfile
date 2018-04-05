FROM alpine:latest
MAINTAINER Yassine Echabbi <github.com/yassine>

ENV GOPATH /go-workspace
ENV PATH $GOPATH/bin:$PATH

RUN mkdir -p $GOPATH/bin
COPY . $GOPATH/src/github.com/yassine/soxy-driver

RUN echo http://dl-3.alpinelinux.org/alpine/latest-stable/community >> /etc/apk/repositories \
    && echo http://dl-3.alpinelinux.org/alpine/latest-stable/testing >> /etc/apk/repositories \
    && apk update \
    && apk upgrade \
    # Permanent Deps
    && apk add --no-cache iptables redsocks tor \
    # Build Deps
    && apk add --no-cache --virtual .soxy-build-deps \
            ca-certificates \
            curl \
		        go \
		        git \
		        gcc \
		        libc-dev \
		        libgcc \
    && curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh \
    && cd $GOPATH/src/github.com/yassine/soxy-driver \
    && echo "Fetching go dependencies, this may take some time" \
    && dep ensure \
    && echo "Dependencies successfully fetched." \
    && go build -o /usr/bin/soxy-driver . \
    && apk del .soxy-build-deps \
    && rm -rf $GOPATH \
    && rm -rf /var/cache/apk/*

ENTRYPOINT [ "soxy-driver" ]
