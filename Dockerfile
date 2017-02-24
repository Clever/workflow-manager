FROM alpine:3.3

RUN apk-install ca-certificates
COPY build/workflow-manager /bin/workflow-manager

CMD ["/bin/workflow-manager", "--addr=0.0.0.0:80"]

