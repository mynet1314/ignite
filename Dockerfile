FROM golang:1.9 as builder
ARG VERSION
WORKDIR /go/src/github.com/mynet1314/nlan
COPY . .
RUN go get -v
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=${VERSION}" -o nlan .


FROM alpine
LABEL maintainer="mynet1314"
RUN apk --no-cache add ca-certificates tzdata sqlite \
			&& cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
			&& echo "Asia/Shanghai" >  /etc/timezone \
			&& apk del tzdata
# See https://stackoverflow.com/questions/34729748/installed-go-binary-not-found-in-path-on-alpine-linux-docker
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
VOLUME /root/nlan/data

WORKDIR /root/nlan
COPY --from=builder /go/src/github.com/mynet1314/nlan/nlan ./
COPY --from=builder /go/src/github.com/mynet1314/nlan/templates ./templates
COPY --from=builder /go/src/github.com/mynet1314/nlan/static ./static
COPY --from=builder /go/src/github.com/mynet1314/nlan/conf ./conf
RUN mv ./conf/config-temp.toml ./conf/config.toml

EXPOSE 5000
ENTRYPOINT ["./nlan"]
