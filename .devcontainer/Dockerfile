FROM mcr.microsoft.com/devcontainers/go:1-1.22-bookworm

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends \
    postgresql-client wget curl

# flyctl
RUN curl -L https://fly.io/install.sh | FLYCTL_INSTALL=/usr/local sh

# Tailwind CLI
RUN wget -O /usr/local/bin/tailwindcss https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-linux-x64
RUN chmod +x /usr/local/bin/tailwindcss
