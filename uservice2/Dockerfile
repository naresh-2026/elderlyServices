# syntax=docker/dockerfile:1
# escape=\
# check=error=true

FROM golang:alpine AS builder
WORKDIR /uservice2
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM scratch
COPY --from=builder /main .
COPY static ./static
EXPOSE 8181
CMD ["/main"] 
