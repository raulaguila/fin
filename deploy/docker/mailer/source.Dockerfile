FROM golang:1.25 AS builder

WORKDIR /opt/authenticity

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY cmd/ cmd/
COPY configs/locales configs/locales
COPY configs/configs.go configs/configs.go
COPY configs/version.txt configs/version.txt
COPY docs/ docs/
COPY internal/ internal/
COPY pkg/ pkg/
COPY Makefile Makefile

RUN make build-mailer


FROM alpine:3.22.1

RUN apk add --no-cache bash tzdata dumb-init

WORKDIR /opt/authenticity

# Secrets via compose env_file — do not copy configs/.env into the image.
COPY --from=builder /opt/authenticity/bin/mailer .

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./mailer" ]
