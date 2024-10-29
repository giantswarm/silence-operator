FROM alpine:3.20.3

RUN apk add --no-cache ca-certificates

ADD ./silence-operator /silence-operator

ENTRYPOINT ["/silence-operator"]
