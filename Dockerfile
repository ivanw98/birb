# syntax=docker/dockerfile:1

# --- SPA ---------------------------------------------------------------------
FROM node:24-alpine AS web
WORKDIR /web
# Pinned rather than corepack-resolved: package.json declares no packageManager.
RUN npm install -g pnpm@11.10.0
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
# Note: VITE_ keys are intentionally exposed to client JS and are safe to pass as ARGs
ARG VITE_SUPABASE_URL
ARG VITE_SUPABASE_PUBLISHABLE_KEY
# A blank URL makes createClient() throw: a white screen with nothing in the logs.
RUN test -n "$VITE_SUPABASE_URL" && test -n "$VITE_SUPABASE_PUBLISHABLE_KEY" \
    || (echo "build args VITE_SUPABASE_URL and VITE_SUPABASE_PUBLISHABLE_KEY are required" >&2; exit 1)
ENV VITE_SUPABASE_URL=$VITE_SUPABASE_URL \
    VITE_SUPABASE_PUBLISHABLE_KEY=$VITE_SUPABASE_PUBLISHABLE_KEY
RUN pnpm run build

# --- API ---------------------------------------------------------------------
FROM golang:1.26-alpine AS api
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Only what ./cmd/api needs, so a frontend edit never busts the Go layer cache.
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY db/ ./db/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -buildvcs=false \
    -ldflags="-s -w" -o /out/birb ./cmd/api

# --- runtime -----------------------------------------------------------------
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -H -u 10001 birb
WORKDIR /app
COPY --from=api /out/birb /app/birb
COPY --from=web /web/dist /app/web
ENV PORT=8080 STATIC_DIR=/app/web
EXPOSE 8080
USER birb
ENTRYPOINT ["/app/birb"]
