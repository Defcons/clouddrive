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
RUN CGO_ENABLED=0 GOOS=linux go build -o /clouddrive .

# Stage 3: Final image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-build /clouddrive .

EXPOSE 8080
VOLUME ["/data"]

ENV STORAGE_ROOT=/data
ENV PORT=8080

CMD ["./clouddrive"]
