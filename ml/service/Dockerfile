# syntax=docker/dockerfile:1
# Anomaly scorer. Build context: repository root (needs ml/anomaly, ml/service,
# the legacy generator and docs/03_architecture/schemas):
#   docker build -f ml/service/Dockerfile -t examory/anomaly-scorer:dev .
# The root .dockerignore allow-lists only those paths.
FROM python:3.12.16-slim-bookworm AS build
ENV PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1
COPY ml/service/requirements.txt /tmp/requirements.txt
RUN python -m pip install --prefix=/install -r /tmp/requirements.txt

FROM python:3.12.16-slim-bookworm
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 PYTHONPATH=/app \
    ANOMALY_SCHEMA_DIR=/app/schemas MPLCONFIGDIR=/tmp HOME=/tmp
RUN groupadd --system --gid 10001 app && useradd --system --uid 10001 --gid app --no-create-home app
COPY --from=build /install /usr/local
WORKDIR /app
COPY ml/anomaly ./ml/anomaly
COPY ml/service ./ml/service
COPY ml/datasets/generate_quiz_behavior.py ./ml/datasets/generate_quiz_behavior.py
COPY docs/03_architecture/schemas ./schemas
USER 10001:10001
EXPOSE 8081
ENTRYPOINT ["python", "-m", "ml.service"]
