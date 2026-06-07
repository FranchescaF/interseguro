# Use official Node.js LTS on Alpine (small image)
FROM node:20-alpine

WORKDIR /app

# Install dependencies first to leverage Docker layer caching
COPY package.json ./
RUN npm install --production

# Copy application source
COPY index.js .

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3001/health',(r)=>process.exit(r.statusCode===200?0:1)).on('error',()=>process.exit(1))"

CMD ["node", "index.js"]
