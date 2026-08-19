FROM alpine:3.22.1

RUN apk add --no-cache bash tzdata dumb-init

WORKDIR /opt/authenticity

# Secrets come from compose env_file / runtime env — do not bake configs/.env into the image.
COPY bin/mailer .

RUN chmod +x mailer

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./mailer" ]
