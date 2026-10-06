FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app ./app

EXPOSE 8080

# uvloop event loop and httptools parser. The number of processes comes from
# WEB_CONCURRENCY (uvicorn's default for --workers); each process runs its
# own job consumer.
CMD ["uvicorn", "app.main:application", "--host", "0.0.0.0", "--port", "8080", "--loop", "uvloop", "--http", "httptools"]
