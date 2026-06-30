FROM python:3.12-slim

WORKDIR /app
COPY services/kuzu-bridge/ ./
RUN pip install --no-cache-dir .

ENV KUZU_ENABLED=true
ENV KUZU_DB_PATH=/data/kuzu/memory_graph.kuzu
VOLUME ["/data/kuzu"]
EXPOSE 8093

CMD ["uvicorn", "quark_kuzu_bridge.main:app", "--host", "0.0.0.0", "--port", "8093"]
