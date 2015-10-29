## ETCD Stress

### Context
While running ETCD in the Diego project of Cloud Foundry, we've noticed problems when we insert many records into the store. We've seen the ETCD nodes apparently failing to communicate with each other and rapidly increasing the raft term. During these time periods, some writes fail.

In the logs, we see many messages like:

```
2015/10/29 18:56:40 etcdhttp: got unexpected response error (etcdserver: request timed out)
```

For a larger chunk of logs, see logs.txt in this repo.

Our environment:

 - We run a 3-node ETCD cluster
 - We have both peer and client/server SSL enabled.
 - Our election timeout is 2000ms and our hearbeat interval is 200ms.
 - Our ETCD nodes are hosted on c4.4xlarge VMs in AWS (30 GB memory, 16 cores), each using a 10gb magnetic disk for the ETCD store.

### Compilation
```bash
go get github.com/jenspinney/etcdstress
```

### Running
To reproduce this issue on our environment, we `etcdstress` with `-dataCountRequested` (i.e., the number of records to insert) set to 200,000, and we started seeing our degradation case when approximately 120,000 records had been inserted. These numbers will likely vary depending on the environment.

```bash
$GOPATH/bin/etcdstress \
  -dataCountRequested=200000 \
  -numPopulateWorkers=50 \
  -etcdCluster=<comma separated cluster IPs> \
  -etcdCertFile=<client cert file here> \
  -etcdKeyFile=<client key file here>
```
