# build
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /todox ./cmd/todox

# run
FROM alpine:3.20
RUN apk add --no-cache git
COPY --from=builder /todox /usr/local/bin/todox
WORKDIR /repo
EXPOSE 8080
# CLI: docker run --rm -v $PWD:/repo image todox --help
# Web: docker run --rm -p 8080:8080 -v $PWD:/repo image todox serve -p 8080
ENTRYPOINT ["todox"]
