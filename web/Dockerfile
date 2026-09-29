# syntax=docker/dockerfile:1

# web image. Node 22 LTS everywhere, regardless of the host's version.
#   dev      -- next dev, source bind-mounted by Compose
#   builder  -- next build with output: "standalone"
#   runtime  -- the standalone server, non-root (the default target)

FROM node:22-alpine AS base
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

# --- development -----------------------------------------------------------
FROM base AS dev
COPY . .
EXPOSE 3000
CMD ["npm", "run", "dev"]

# --- build -----------------------------------------------------------------
FROM base AS builder
COPY . .
RUN npm run build

# --- runtime ---------------------------------------------------------------
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

USER node
EXPOSE 3000
CMD ["node", "server.js"]
