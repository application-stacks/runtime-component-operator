FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

LABEL vendor="Application Stacks" \
      name="Application Stacks Operator" \
      version="0.4.0" \
      summary="Image for Application Stacks Operator" \
      description="This image contains the controller for Application Stacks Operator. See https://github.com/application-stacks/operator"
      
ENV OPERATOR=/usr/local/bin/application-stacks-operator \
    USER_UID=1001 \
    USER_NAME=application-stacks-operator

# install operator binary
COPY build/_output/bin/operator ${OPERATOR}
COPY build/bin /usr/local/bin
COPY LICENSE /licenses/

RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
