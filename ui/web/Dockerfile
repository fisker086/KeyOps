# Frontend Dockerfile
# 构建前端并配置 Nginx 反向代理

# ---------- Stage 1: Build frontend (Vite/React) ----------
FROM node:20-alpine AS frontend-builder
WORKDIR /build

# Copy package files first for better cache
COPY package*.json ./

# Install dependencies
RUN npm ci --no-audit --no-fund

# Copy source code
COPY . .

# Build frontend
RUN npm run build

# ---------- Stage 2: Runtime (nginx) ----------
FROM nginx:alpine

WORKDIR /app

# 使用阿里云镜像源加速下载
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk update && \
    apk add --no-cache tzdata wget

# Copy frontend build to nginx html
COPY --from=frontend-builder /build/dist /usr/share/nginx/html

# Copy nginx config (will be provided by docker-compose or mounted)
# Note: nginx.conf should be copied from parent directory or mounted as volume
# COPY ../nginx.conf /etc/nginx/conf.d/default.conf

# Create logs directory
RUN mkdir -p /app/logs

# Expose port 80
EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1/ >/dev/null 2>&1 || exit 1

# Start nginx
CMD ["nginx", "-g", "daemon off;"]

