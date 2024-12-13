#!/bin/bash

go install honnef.co/go/tools/cmd/staticcheck@latest

cp -n .env.public .env
source .env

make audit
