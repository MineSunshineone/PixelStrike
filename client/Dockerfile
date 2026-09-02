FROM node:22-alpine AS builder
WORKDIR /app/client
COPY client/package.json client/package-lock.json ./
RUN npm ci
COPY client ./
COPY map.json /app/map.json
COPY maps/neon-city.json /app/maps/neon-city.json
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/client/dist /usr/share/nginx/html
COPY map.json /usr/share/nginx/html/map.json
COPY client/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
