FROM golang:1.26 AS builder

WORKDIR /opt/authenticity

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY cmd/ cmd/
COPY config/ config/
COPY internal/ internal/
COPY pkg/ pkg/
COPY Makefile Makefile

RUN make build-backend build-admin


FROM alpine:3.22.1

RUN apk add --no-cache bash tzdata dumb-init

WORKDIR /opt/authenticity

# Secrets via compose env_file — do not copy config/.env into the image.
COPY --from=builder /opt/authenticity/bin/backend /opt/authenticity/bin/admin ./

USER nobody

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./backend" ]
