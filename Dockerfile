FROM golang:1.19-alpine as build-env
RUN apk add --no-cache bash
RUN addgroup -S nginx && adduser -S nginx -G nginx

WORKDIR /app

COPY . ./

RUN go mod download

RUN go build -o /upload-service

EXPOSE 3000
USER nginx
CMD [ "/upload-service" ]
