FROM alpine:3.22.1

RUN apk add --no-cache bash tzdata dumb-init

WORKDIR /opt/authenticity

# Secrets come from compose env_file — do not bake config/.env into the image.
COPY bin/backend bin/admin ./

RUN chmod +x backend admin

USER nobody

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./backend" ]
