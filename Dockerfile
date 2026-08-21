# Stage 1: Build frontend
FROM node:20-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Stage 2: Build backend
FROM golang:1.22-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
# Copy built frontend into Go embed directory
RUN mkdir -p static
COPY --from=frontend-build /app/frontend/dist/ ./static/
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /clouddrive .

# Stage 3: Final image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
# Run as non-root. uid/gid 1000 matches Syncthing's PUID/PGID so both containers
# share the /data volume cleanly. REQUIRES the host /data to be owned by 1000:1000
# (run `chown -R 1000:1000 /data` on the host once) or the app can't write and the
# container fails on startup.
RUN addgroup -g 1000 app && adduser -D -u 1000 -G app app
WORKDIR /app
COPY --from=backend-build /clouddrive .
USER app

EXPOSE 8080
VOLUME ["/data"]

ENV STORAGE_ROOT=/data
ENV PORT=8080

CMD ["./clouddrive"]
