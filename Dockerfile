# build
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# builda o binário do package main na raiz
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/app .

# runtime
FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=build /out/app /app
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app"]
