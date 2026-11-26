FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/ ./cmd/...

FROM alpine:3.22
ARG SERVICE
COPY --from=build /out/${SERVICE} /usr/local/bin/ecommerce
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/ecommerce"]
