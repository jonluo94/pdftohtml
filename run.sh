#!/bin/bash

export NAME=docd
export VERSION=debian

echo "Building ${NAME} for ${VERSION}..."

GOOS=linux GOARCH=amd64 go build -o $NAME || exit 1
docker build -t $NAME . || exit 1

# docker load -i docd.tar

docker run -d --name docconv \
          -p 8888:8888  \
          docd:latest

echo "run docd success"