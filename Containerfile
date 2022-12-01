FROM docker.io/library/golang:1.19 AS builder
WORKDIR /workspace
COPY go.mod ./
COPY go.sum ./
COPY main.go ./
COPY main_test.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o aws-s3-bucket-cleaner

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/aws-s3-bucket-cleaner /aws-s3-bucket-cleaner
ENTRYPOINT ["/bucket-cleaner"]
