# Deployment Guide

## Quick Start (Binary)

```bash
# Build
git clone https://github.com/Mariusrfx/logtailr.git
cd logtailr
make build-all    # Frontend + Go binary

# Run
./bin/logtailr tail --config config.yaml --api --web
# Open http://localhost:8080
```

---

## Docker

### Build image

```dockerfile
# Dockerfile
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o logtailr .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=backend /app/logtailr /usr/local/bin/logtailr
EXPOSE 8080
ENTRYPOINT ["logtailr"]
```

```bash
docker build -t logtailr:latest .
```

### Run with Docker

```bash
# Minimal: tail a file
docker run -v /var/log:/var/log:ro logtailr:latest \
  tail --file /var/log/syslog --api --web --api-addr 0.0.0.0

# With config file
docker run -v /var/log:/var/log:ro \
  -v ./config.yaml:/etc/logtailr/config.yaml:ro \
  -p 8080:8080 \
  logtailr:latest \
  tail --config /etc/logtailr/config.yaml --api --web --api-addr 0.0.0.0

# With PostgreSQL
docker run -v /var/log:/var/log:ro \
  -e LOGTAILR_DB_URL="postgres://user:pass@host:5432/logtailr?sslmode=disable" \
  -e LOGTAILR_API_TOKEN="my-secret-token" \
  -p 8080:8080 \
  logtailr:latest \
  tail --api --web --api-addr 0.0.0.0
```

### Docker Compose (full stack)

```yaml
# docker-compose.yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: logtailr
      POSTGRES_USER: logtailr
      POSTGRES_PASSWORD: secret
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U logtailr"]
      interval: 5s
      retries: 5

  logtailr:
    build: .
    ports:
      - "8080:8080"
    environment:
      LOGTAILR_DB_URL: "postgres://logtailr:secret@postgres:5432/logtailr?sslmode=disable"
      LOGTAILR_API_TOKEN: "change-me-in-production"
    volumes:
      - /var/log:/var/log:ro
      - ./config.yaml:/etc/logtailr/config.yaml:ro
    depends_on:
      postgres:
        condition: service_healthy
    command: >
      tail --config /etc/logtailr/config.yaml
           --api --web --api-addr 0.0.0.0

volumes:
  pgdata:
```

```bash
docker compose up -d

# Run migrations
docker compose exec logtailr logtailr migrate up

# Import config
docker compose exec logtailr logtailr import --config-file /etc/logtailr/config.yaml

# Open dashboard
open http://localhost:8080
```

---

## Systemd

### Install binary

```bash
sudo cp bin/logtailr /usr/local/bin/
sudo chmod +x /usr/local/bin/logtailr
```

### Create config

```bash
sudo mkdir -p /etc/logtailr
sudo cp config.yaml /etc/logtailr/config.yaml
```

### Create systemd service

```ini
# /etc/systemd/system/logtailr.service
[Unit]
Description=Logtailr Log Aggregator
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=logtailr
Group=logtailr
ExecStart=/usr/local/bin/logtailr tail \
  --config /etc/logtailr/config.yaml \
  --api --web --api-addr 0.0.0.0 --api-port 8080
Restart=on-failure
RestartSec=5

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadOnlyPaths=/var/log
ReadWritePaths=/var/lib/logtailr

# Environment
Environment=LOGTAILR_DB_URL=postgres://logtailr:secret@localhost:5432/logtailr?sslmode=disable
Environment=LOGTAILR_API_TOKEN=change-me
# Or use an env file:
# EnvironmentFile=/etc/logtailr/env

[Install]
WantedBy=multi-user.target
```

### Create user and enable

```bash
sudo useradd -r -s /usr/sbin/nologin logtailr
sudo mkdir -p /var/lib/logtailr
sudo chown logtailr:logtailr /var/lib/logtailr

sudo systemctl daemon-reload
sudo systemctl enable logtailr
sudo systemctl start logtailr
sudo systemctl status logtailr

# View logs
journalctl -u logtailr -f
```

---

## Kubernetes

### Deployment manifest

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: logtailr
  labels:
    app: logtailr
spec:
  replicas: 1
  selector:
    matchLabels:
      app: logtailr
  template:
    metadata:
      labels:
        app: logtailr
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: logtailr
      containers:
        - name: logtailr
          image: logtailr:latest
          args:
            - tail
            - --api
            - --web
            - --api-addr=0.0.0.0
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: LOGTAILR_DB_URL
              valueFrom:
                secretKeyRef:
                  name: logtailr-secrets
                  key: db-url
            - name: LOGTAILR_API_TOKEN
              valueFrom:
                secretKeyRef:
                  name: logtailr-secrets
                  key: api-token
          volumeMounts:
            - name: config
              mountPath: /etc/logtailr
              readOnly: true
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 5
          readinessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 3
      volumes:
        - name: config
          configMap:
            name: logtailr-config
---
apiVersion: v1
kind: Service
metadata:
  name: logtailr
spec:
  selector:
    app: logtailr
  ports:
    - port: 8080
      targetPort: http
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: logtailr-config
data:
  config.yaml: |
    sources:
      - name: "k8s-api"
        type: "kubernetes"
        namespace: "default"
        label_selector: "app=my-app"
        follow: true
        parser: "json"
    global:
      level: "info"
---
apiVersion: v1
kind: Secret
metadata:
  name: logtailr-secrets
type: Opaque
stringData:
  db-url: "postgres://logtailr:secret@postgres:5432/logtailr?sslmode=disable"
  api-token: "change-me-in-production"
```

### Apply

```bash
kubectl apply -f k8s/
kubectl port-forward svc/logtailr 8080:8080
open http://localhost:8080
```

---

## Reverse Proxy (Nginx)

```nginx
# /etc/nginx/sites-available/logtailr
server {
    listen 443 ssl;
    server_name logs.example.com;

    ssl_certificate     /etc/letsencrypt/live/logs.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/logs.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket support
    location /ws/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_read_timeout 86400;
    }
}
```

---

## Production Checklist

- [ ] Set `LOGTAILR_API_TOKEN` to a strong random value
- [ ] Use TLS (reverse proxy or direct)
- [ ] Set `--api-addr 127.0.0.1` if behind reverse proxy
- [ ] Configure PostgreSQL with proper credentials and `sslmode=require`
- [ ] Run `logtailr migrate up` before first start
- [ ] Set `--log-format json` for structured logging to observability stack
- [ ] Configure alert notification channels (webhook/email)
- [ ] Set up Prometheus scraping on `/metrics` endpoint
- [ ] Monitor logtailr itself with systemd watchdog or Kubernetes probes
- [ ] Review CORS settings if dashboard is accessed from different domain
