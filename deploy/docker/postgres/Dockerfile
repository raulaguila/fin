FROM postgres:17.2-alpine

RUN apk add --no-cache postgresql-pg_cron \
    && cp /usr/lib/postgresql*/pg_cron.so /usr/local/lib/postgresql/ \
    && cp /usr/share/postgresql*/extension/pg_cron* /usr/local/share/postgresql/extension/

RUN echo "shared_preload_libraries = 'pg_cron'" >> /usr/local/share/postgresql/postgresql.conf.sample
RUN echo "cron.database_name = '${POSTGRES_BASE}'" >> /usr/local/share/postgresql/postgresql.conf.sample
