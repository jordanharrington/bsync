FROM golang:1.24 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY aws .
RUN CGO_ENABLED=0 GOOS= GOARCH= \
    go build -trimpath -ldflags="-s -w" -o /bin/aws-gateway ./cmd/aws-gateway/main.go

FROM public.ecr.aws/lambda/provided:al2023

COPY --from=build /bin/aws-gateway .

ENTRYPOINT["/aws-gateway"]
