ARG PG_BASE_IMAGE=postgres:17
FROM ${PG_BASE_IMAGE}

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       python3 python3-pip python3-venv python3-dev gcc libpq-dev \
    && python3 -m venv /opt/patroni \
    && /opt/patroni/bin/pip install --no-cache-dir patroni[etcd3] psycopg2-binary \
    && apt-get purge -y python3-dev gcc \
    && apt-get autoremove -y \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

ENV PATH="/opt/patroni/bin:${PATH}"

COPY patroni-entrypoint.sh /usr/local/bin/patroni-entrypoint.sh
RUN chmod +x /usr/local/bin/patroni-entrypoint.sh

EXPOSE 5432 8008

USER postgres
ENTRYPOINT ["/usr/local/bin/patroni-entrypoint.sh"]
