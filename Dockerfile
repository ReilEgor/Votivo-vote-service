FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG COMMIT=dev
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.commit=${COMMIT}" -o /out/vote ./cmd/vote

FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/vote /usr/local/bin/vote
EXPOSE 8080
ENTRYPOINT ["vote"]
