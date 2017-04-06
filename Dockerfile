FROM alpine:3.3

RUN apk update && apk add ca-certificates
COPY build/workflow-manager /bin/workflow-manager

CMD ["/bin/workflow-manager", "--addr=0.0.0.0:80"]

