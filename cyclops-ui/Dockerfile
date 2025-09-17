FROM node:20.10.0-alpine AS builder
WORKDIR /app

COPY package.json ./
COPY package-lock.json ./  

RUN npm config set registry https://registry.npmjs.org/
    

RUN npm install --legacy-peer-deps
    

COPY . .

ARG VERSION=0.0.0
ENV VITE_APP_VERSION=$VERSION
ENV NODE_OPTIONS=--max-old-space-size=1536  

RUN npm run build
    
FROM nginx:stable-alpine AS app
WORKDIR /usr/share/nginx/html  

RUN apk add --no-cache gettext
    

COPY nginx-entrypoint.sh /
COPY env.js ./
    

COPY --from=builder /app/build ./
    

COPY ./nginx/default.conf /etc/nginx/conf.d/default.conf
    

ARG VERSION
ENV REACT_APP_VERSION=$VERSION
    
ENTRYPOINT ["sh", "/nginx-entrypoint.sh"]
    