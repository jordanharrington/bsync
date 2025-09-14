ARG OS=linux
ARG ARCH=amd64
FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH \
    go build -trimpath -ldflags="-s -w" -o gateway ./cmd/aws-gateway/main.go

FROM public.ecr.aws/lambda/provided:al2023

COPY --from=builder /src/gateway ./gateway

ENTRYPOINT [ "./gateway" ]