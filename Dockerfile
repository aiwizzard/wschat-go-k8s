FROM golang:1.21.0

RUN mkdir -p /app

WORKDIR /app

ADD main /app/main

EXPOSE 8080

CMD [ "./main" ]