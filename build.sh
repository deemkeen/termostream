#!/bin/bash

if [ ! -f private.pem ] || [ ! -f certificate.pem ]; then
    openssl req -x509 -newkey rsa:2048 -keyout private.pem -out certificate.pem -days 3650 -nodes
fi

go install

go build -ldflags="-s -w"
