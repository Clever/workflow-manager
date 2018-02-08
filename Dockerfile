FROM alpine:3.3

RUN apk update && apk add ca-certificates
COPY bin/workflow-manager /bin/workflow-manager
COPY kvconfig.yml /bin/kvconfig.yml

CMD ["/bin/workflow-manager", "--addr=0.0.0.0:80"]

