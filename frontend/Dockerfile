FROM node:24-alpine AS build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
COPY infra/next-csp.mjs /tmp/next-csp.mjs
RUN NEXT_TELEMETRY_DISABLED=1 npm run build && node /tmp/next-csp.mjs out /tmp/next-csp.conf

FROM nginx:1.28-alpine
COPY infra/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /tmp/next-csp.conf /etc/nginx/conf.d/00-next-csp.conf
COPY --from=build /src/out/ /usr/share/nginx/html/
COPY ["Code Challenge E-Commerce.csv", "/usr/share/nginx/html/sample-products.csv"]
EXPOSE 8080
