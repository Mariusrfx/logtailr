# ---- web: build the dashboard ----
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- go: build the binary with the dashboard embedded ----
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY --from=web /src/web/dist ./internal/web/dist
ARG VERSION=v0.0.0
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X logtailr/cmd.version=${VERSION}" -o /out/logtailr .

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S logtailr && adduser -S -G logtailr logtailr
USER logtailr
COPY --from=build /out/logtailr /usr/local/bin/logtailr
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/logtailr", "tail"]
