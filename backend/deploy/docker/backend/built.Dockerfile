FROM alpine:3.22.1

RUN sed -i 's|https://dl-cdn.alpinelinux.org|http://dl-cdn.alpinelinux.org|g' /etc/apk/repositories && \
    apk add --no-cache bash tzdata dumb-init nodejs npm libgcc libstdc++ ripgrep && \
    npm install -g @anthropic-ai/claude-code@2.1.186 && npm cache clean --force && \
    mkdir -p /home/fin && chown nobody /home/fin

# Claude Code runs the assistant when AI_PROVIDER=claude (CLAUDE_CODE_OAUTH_TOKEN in env_file).
ENV HOME=/home/fin USE_BUILTIN_RIPGREP=0

WORKDIR /opt/fin

# Secrets come from compose env_file — do not bake config/.env into the image.
COPY bin/backend bin/admin ./

RUN chmod +x backend admin

USER nobody

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]

CMD [ "./backend" ]
