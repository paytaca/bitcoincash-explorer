#
# Production image for Nuxt (Nitro) server
#

FROM node:20-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:20-alpine AS build
WORKDIR /app
ARG SITE_URL
ARG NUXT_PUBLIC_SITE_URL
ENV SITE_URL=$SITE_URL
ENV NUXT_PUBLIC_SITE_URL=$NUXT_PUBLIC_SITE_URL
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NITRO_HOST=0.0.0.0
ENV NITRO_PORT=8000

# Only ship the built output
COPY --from=build /app/.output ./.output

EXPOSE 8000
CMD ["node", ".output/server/index.mjs"]

