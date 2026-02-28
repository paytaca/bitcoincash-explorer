export const config = {
  // ZMQ
  zmqHost: process.env.BCH_ZMQ_HOST || '127.0.0.1',
  zmqPort: parseInt(process.env.BCH_ZMQ_PORT || '28332', 10),
  
  // Redis
  redisUrl: process.env.REDIS_URL || 'redis://127.0.0.1:6379/0',
  redisPrefix: process.env.REDIS_PREFIX || 'bch',
  
  // RPC (for initial sync and fallback)
  rpcUrl: process.env.BCH_RPC_URL || 'http://127.0.0.1:8332',
  rpcUser: process.env.BCH_RPC_USER || '',
  rpcPass: process.env.BCH_RPC_PASS || '',
  
  // Cache settings
  maxBlocks: 15,
  maxTransactions: 20,
  
  // Logging
  logLevel: process.env.LOG_LEVEL || 'info'
}

export function getRedisKey(key: string): string {
  return `${config.redisPrefix}:${key}`
}