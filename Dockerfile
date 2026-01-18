FROM golang:1.22-alpine

WORKDIR /app

COPY ./src ./src

WORKDIR /app/src

RUN go build -o loadbalancer .

EXPOSE 8000

CMD ["./loadbalancer"]
