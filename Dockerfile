FROM scratch
COPY mailalive-exporter /
ENTRYPOINT ["/mailalive-exporter", "/etc/mailalive-exporter.toml"]
