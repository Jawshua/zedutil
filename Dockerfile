FROM golang:1.21-alpine as build
ADD . /go/zedutil
RUN cd /go/zedutil && CGO_ENABLED=0 go build -o zedutil main.go

FROM alpine:3.18
RUN adduser -u 1001 zeduser -D -H -s nologin
COPY --from=build /go/zedutil/zedutil /usr/bin/zedutil
USER 1001
ENTRYPOINT [ "/usr/bin/zedutil" ]
