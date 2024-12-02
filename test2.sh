#!/bin/zsh
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: task-pv-pod
  namespace: default
  labels:
    type: demo
spec:
  volumes:
    - name: task-pv-storage
      persistentVolumeClaim:
        claimName: source-pvc
  containers:
    - name: task-pv-container
      image: nginx
      ports:
        - containerPort: 80
          name: "http-server"
      volumeMounts:
        - mountPath: "/data"
          name: task-pv-storage
EOF
for i in {1..2}; do
  namespace="namespace${i}"
  cat <<EOF | kubectl apply -f -
  apiVersion: v1
  kind: Pod
  metadata:
    name: task-pv-pod
    namespace: ${namespace}
    labels:
      type: demo
  spec:
    volumes:
      - name: task-pv-storage
        persistentVolumeClaim:
          claimName: cephfs-pvc-${namespace}
    containers:
      - name: task-pv-container
        image: nginx
        ports:
          - containerPort: 80
            name: "http-server"
        volumeMounts:
          - mountPath: "/data"
            name: task-pv-storage
EOF
done
