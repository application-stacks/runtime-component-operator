FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

LABEL vendor="Runtime Component Community" \
      name="Runtime Component Operator" \
      version="0.7.0" \
      summary="Image for Runtime Component Operator" \
      description="This image contains the controller for Runtime Component Operator. See https://github.com/application-stacks/runtime-component-operator"
      
ENV OPERATOR=/usr/local/bin/runtime-component-operator \
    USER_UID=1001 \
    USER_NAME=runtime-component-operator

# install operator binary
COPY build/_output/bin/runtime-component-operator ${OPERATOR}
COPY build/bin /usr/local/bin
COPY LICENSE /licenses/

RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
