## ETCD Stress

```
etcdstress -- \
  -dataCountRequested=5000 \
  -numPopulateWorkers=50 \
  -etcdCluster=https://10.244.16.130:4001 \
  -etcdCertFile=$GOPATH/manifest-generation/bosh-lite-stubs/etcd-certs/client.crt \
  -etcdKeyFile=$GOPATH/manifest-generation/bosh-lite-stubs/etcd-certs/client.key \
```
