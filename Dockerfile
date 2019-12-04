# syntax = docker/dockerfile:experimental

#
# ----- Go Builder Image ------
#
FROM golang:1.13 AS builder

# curl git bash
RUN apt-get update && apt-get install -y --no-install-recommends \
		curl \
		git \
		bash \
	&& rm -rf /var/lib/apt/lists/*


#
# ----- Build and Test Image -----
#
FROM builder as build

# set working directory
RUN mkdir -p /go/src/gtoken
WORKDIR /go/src/gtoken

# copy sources
COPY . .

# build
RUN make


#
# ------ get latest CA certificates
#
FROM alpine:3.10 as certs
RUN apk --update add ca-certificates


#
# ------ gtoken release Docker image ------
#
FROM scratch

# copy CA certificates
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# this is the last commabd since it's never cached
COPY --from=build /go/src/gtoken/.bin/gtoken /gtoken

ENTRYPOINT ["/gtoken"]