FROM mcr.microsoft.com/devcontainers/go:1-1.23-bookworm

ENV CGO_ENABLED=1

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends build-essential wget curl

RUN wget -O /usr/local/bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
RUN chmod +x /usr/local/bin/tailwindcss
