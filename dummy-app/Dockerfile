FROM python:slim

WORKDIR /app

COPY index.html .

ENTRYPOINT ["python3", "-m", "http.server", "8000"]
