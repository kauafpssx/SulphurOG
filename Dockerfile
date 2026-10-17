FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends 7zip ca-certificates wget && \
    ln -s /usr/bin/7zz /usr/local/bin/7z && \
    rm -rf /var/lib/apt/lists/* && \
    useradd -r -u 1001 -d /app sulphurog

WORKDIR /app

COPY configs/ configs/

# Binário construído no host (CGO_ENABLED=0 GOOS=linux GOARCH=amd64)
# Use make build-linux e copie para ./sulphurog
COPY --chown=sulphurog:sulphurog sulphurog /app/sulphurog
RUN chmod +x /app/sulphurog

RUN mkdir -p data/temp && chown -R sulphurog:sulphurog /app

USER sulphurog

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8090/api/health || exit 1

CMD ["/app/sulphurog"]