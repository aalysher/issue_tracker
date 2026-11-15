FROM golang:1.27-alpine AS api-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.24
RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates
COPY --from=api-build /out/api /usr/local/bin/api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
