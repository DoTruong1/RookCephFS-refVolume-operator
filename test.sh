#!/bin/zsh

for i in {1..5}; do
  namespace="namespace${i}"
  kubectl create namespace $namespace

  cat <<EOF | kubectl apply -f -
apiVersion: operator.dotv.home.arpa/v1
kind: RookCephFSRefVol
metadata:
  name: rookcephfsrefvol-${i}
spec:
  cephFsUserSecretName: rook-csi-cephfs-node-user
  datasource:
    pvcInfo:
      pvcName: source-pvc
      namespace: default
    pvcSpec:
      createIfNotExists: true
      storageClassName: rook-cephfs
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 1Gi
  destination:
    pvcInfo:
      pvcName: cephfs-pvc-${namespace}
      namespace: ${namespace}
EOF
done

echo "Namespaces and RookCephFSRefVol resources created successfully."

