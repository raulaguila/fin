FROM golang:1.26.10 AS builder

WORKDIR /opt/fin

COPY go.mod .
COPY go.sum .

RUN git config --global url."https://github.com/golang/".insteadOf "https://go.googlesource.com/" && \
    GOPROXY=direct \
    GIT_SSL_NO_VERIFY=true \
    GOINSECURE='*' \
    go mod download

COPY cmd/ cmd/
COPY config/ config/
COPY internal/ internal/
COPY pkg/ pkg/
COPY Makefile Makefile

RUN make build-backend build-admin


FROM alpine:3.22.1

RUN sed -i 's|https://dl-cdn.alpinelinux.org|http://dl-cdn.alpinelinux.org|g' /etc/apk/repositories && \
    apk add --no-cache bash tzdata dumb-init nodejs npm libgcc libstdc++ ripgrep && \
    npm install -g @anthropic-ai/claude-code@2.1.186 && npm cache clean --force && \
    mkdir -p /home/fin && chown nobody /home/fin

# Claude Code runs the assistant when AI_PROVIDER=claude (CLAUDE_CODE_OAUTH_TOKEN in env_file).
ENV HOME=/home/fin USE_BUILTIN_RIPGREP=0

WORKDIR /opt/fin

# Secrets via compose env_file — do not copy config/.env into the image.
COPY --from=builder /opt/fin/bin/backend /opt/fin/bin/admin ./

USER nobody

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./backend" ]
