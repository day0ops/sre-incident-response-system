FROM node:22-alpine AS builder
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:22-alpine
WORKDIR /app
# CI passes the git tag on a release build (short SHA otherwise, see
# ci.yml) -- exposed at runtime via /api/version so the footer can show
# which build is actually running.
ARG VERSION=dev
ENV APP_VERSION=$VERSION
COPY package.json package-lock.json ./
RUN npm ci --omit=dev
COPY server ./server
COPY tsconfig.json ./
COPY --from=builder /src/dist ./dist
EXPOSE 5174
CMD ["npm", "start"]
