FROM registry.redhat.io/openshift4/ose-operator-registry:v4.12 AS builder

FROM registry.redhat.io/ubi8/ubi-minimal

ARG VERSION_LABEL=1.3.1
ARG RELEASE_LABEL=XX
ARG VCS_REF=0123456789012345678901234567890123456789
ARG VCS_URL="https://github.com/application-stacks/runtime-component-operator"
ARG NAME="runtime-component-operator-catalog"
ARG SUMMARY="Runtime Component Operator Catalog"
ARG DESCRIPTION="This image contains the catalog for Runtime Component Operator."

ARG USER_ID=1001

LABEL name=$NAME \
      vendor=IBM \
      version=$VERSION_LABEL \
      release=$RELEASE_LABEL \
      description=$DESCRIPTION \
      summary=$SUMMARY \
      io.k8s.display-name=$SUMMARY \
      io.k8s.description=$DESCRIPTION \
      vcs-type=git \
      vcs-ref=$VCS_REF \
      vcs-url=$VCS_URL \
      url=$VCS_URL

# Copy Apache license
COPY LICENSE /licenses

COPY --chown=1001:0 bundles.db /database/index.db
LABEL operators.operatorframework.io.index.database.v1=/database/index.db

COPY --from=builder --chown=1001:0 /bin/registry-server /registry-server
COPY --from=builder --chown=1001:0 /bin/grpc_health_probe /bin/grpc_health_probe

EXPOSE 50051

USER ${USER_ID}

WORKDIR /tmp
ENTRYPOINT ["/registry-server"]
CMD ["--database", "/database/index.db"]
