FROM alpine:3.15.1

RUN apk add --no-cache ca-certificates

ADD ./silence-operator /silence-operator

ENTRYPOINT ["/silence-operator"]
