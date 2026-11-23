FROM python:3.12.16-slim-bookworm
ENV PYTHONPATH=/app PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1
WORKDIR /app
COPY ml/integrity_service/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir -r /tmp/requirements.txt
COPY ml/integrity/minhash.py ml/integrity/to_risk_event.py ml/integrity/
COPY ml/service/schema.py ml/service/
COPY ml/integrity_service/*.py ml/integrity_service/
COPY docs/03_architecture/schemas/ schemas/
USER 10001:10001
ENTRYPOINT ["python","-m","ml.integrity_service"]
