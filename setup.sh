#!/bin/bash

go install github.com/air-verse/air@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

cp -n .env.public .env
source .env
