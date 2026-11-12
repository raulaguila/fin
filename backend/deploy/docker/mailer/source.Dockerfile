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

RUN make build-mailer


FROM alpine:3.22.1

RUN sed -i 's|https://dl-cdn.alpinelinux.org|http://dl-cdn.alpinelinux.org|g' /etc/apk/repositories && \
    apk add --no-cache bash tzdata dumb-init

WORKDIR /opt/fin

# Secrets via compose env_file — do not copy config/.env into the image.
COPY --from=builder /opt/fin/bin/mailer .

USER nobody

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./mailer" ]
