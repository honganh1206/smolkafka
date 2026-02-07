#!/bin/sh

# Extract pod ordinal from hostname (e.g., smolkafka-2 -> 2)
# rev reverses string, cut gets first field after '-', rev restores order
ID=$(echo $HOSTNAME | rev | cut -d- -f1 | rev)

# Generate config file using heredoc
cat > /var/run/smolkafka/config.yaml << EOD
datadir: /var/run/smolkafka/data
rpc-port: ${RPC_PORT}
# Construct FQDN for this pod within the cluster
bind-addr: "${HOSTNAME}.smolkafka.${NAMESPACE}.svc.cluster.local:${SERF_PORT}"
# Pod 0 bootstraps the cluster; others join pod 0
bootstrap: $([ $ID = 0 ] && echo true || echo false)
$([ $ID != 0 ] && echo "start-join-addrs: \"smolkafka-0.smolkafka.${NAMESPACE}.svc.cluster.local:${SERF_PORT}\"")
EOD
