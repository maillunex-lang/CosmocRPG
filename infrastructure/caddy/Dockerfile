# syntax=docker/dockerfile:1
# Контекст сборки: корень репозитория.
FROM node:22-alpine AS admin
WORKDIR /admin
COPY admin/package.json admin/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY admin/ ./
RUN npm run build

FROM caddy:2-alpine
COPY --from=admin /admin/dist /srv
COPY site /site
COPY infrastructure/caddy/Caddyfile /etc/caddy/Caddyfile
