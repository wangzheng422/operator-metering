#!/bin/bash

set -e
set -u

cat <<EOF
operator:
  image:
    repository: ${METERING_OPERATOR_IMAGE_REPO}
    tag: ${METERING_OPERATOR_IMAGE_TAG}
EOF

if [ -n "${METERING_OPERATOR_ALL_NAMESPACES:-}" ]; then
    cat <<EOF
  allNamespaces: "$METERING_OPERATOR_ALL_NAMESPACES"
EOF
fi

if [ -n "${METERING_OPERATOR_TARGET_NAMESPACES:-}" ]; then
    cat <<EOF
  targetNamespaces: [ $METERING_OPERATOR_TARGET_NAMESPACES ]
EOF
fi
