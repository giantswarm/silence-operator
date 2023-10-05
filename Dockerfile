FROM alpine:3.18.4

RUN apk add --no-cache ca-certificates

ADD ./silence-operator /silence-operator

ENTRYPOINT ["/silence-operator"]
