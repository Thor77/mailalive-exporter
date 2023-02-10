FROM alpine
COPY mailalive-exporter /
ENTRYPOINT ["/mailalive-exporter", "/etc/mailalive-exporter.toml"]
