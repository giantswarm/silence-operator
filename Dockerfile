FROM alpine:3.19.0

RUN apk add --no-cache ca-certificates

ADD ./silence-operator /silence-operator

ENTRYPOINT ["/silence-operator"]
