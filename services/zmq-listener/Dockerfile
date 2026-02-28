FROM node:20-alpine

WORKDIR /app

# Install build dependencies and runtime libraries for zeromq
RUN apk add --no-cache python3 make g++ linux-headers libzmq

# Install dependencies (using install instead of ci since no lock file exists)
COPY package.json ./
RUN npm install

# Copy source and build
COPY tsconfig.json ./
COPY src/ ./src/
RUN npm run build

# Remove build dependencies only (keep runtime libraries)
RUN apk del python3 make g++ linux-headers

# Create non-root user
RUN addgroup -g 1001 -S nodejs && \
    adduser -S zmq-listener -u 1001 && \
    chown -R zmq-listener:nodejs /app

USER zmq-listener

EXPOSE 3000

CMD ["npm", "start"]