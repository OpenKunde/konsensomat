# Runtime image built from the pre-compiled artifact (see: make docker-build).
# Uses scratch — no OS, no shell. Templates and static assets are already
# compiled into the binary via go:embed, so there's nothing else to copy.
#
# `scratch` has no mkdir/chown, so a tiny alpine stage prepares an empty,
# correctly-owned data directory that gets copied into the final image -
# without it, the nonroot user below couldn't create KONSENSOMAT_DATA_DIR
# itself at startup.
FROM alpine:3.20 AS prepare
RUN mkdir -p /data && chown 65532:65532 /data

FROM scratch

WORKDIR /app

# UID 65532 = nonroot convention (no adduser available in scratch)
COPY --from=prepare --chown=65532:65532 /data ./files/data
COPY --chown=65532:65532 konsensomat ./konsensomat

USER 65532:65532

EXPOSE 8080
VOLUME ["/app/files/data"]

CMD ["./konsensomat"]
