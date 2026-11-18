FROM python:3.13-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY schema.sql seed.py engine.py api.py ./
RUN mkdir -p /app/data
ENV HR_DB_PATH=/app/data/hr.sqlite3
ENV APP_ENV=demo
EXPOSE 8080
CMD ["sh", "-c", "test -s /app/data/hr.sqlite3 || python seed.py --db /app/data/hr.sqlite3; exec uvicorn api:app --host 0.0.0.0 --port 8080"]
