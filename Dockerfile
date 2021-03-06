# build
FROM golang:1.12-alpine AS build
RUN apk add git
ADD . /src
WORKDIR /src
RUN go build -o app

# app
FROM alpine
WORKDIR /srv
RUN mkdir /data
EXPOSE 80
ENV KEY="1"
ENV DELAY=10
ENV DB="/data/stat.db"
COPY --from=build /src/app ./
ENTRYPOINT ["/srv/app"]
