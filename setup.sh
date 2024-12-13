#!/bin/bash

go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install honnef.co/go/tools/cmd/staticcheck@latest

cp -n .env.public .env
source .env

migrate -path ./migrations -database $DATABASE_URL up
