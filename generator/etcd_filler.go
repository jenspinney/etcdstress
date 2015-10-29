package generator

import (
	"fmt"
	"sync"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/coreos/go-etcd/etcd"
	"github.com/nu7hatch/gouuid"
)

const ERROR_TOLERANCE = 0.05

type EtcdFiller struct {
	etcdClient *etcd.Client
	workPool   *workpool.WorkPool
}

func NewEtcdFiller(
	workpoolSize int,
	etcdClient *etcd.Client,
) *EtcdFiller {
	workPool, err := workpool.NewWorkPool(workpoolSize)
	if err != nil {
		panic(err)
	}
	return &EtcdFiller{
		etcdClient: etcdClient,
		workPool:   workPool,
	}
}

func (g *EtcdFiller) Generate(count int) (int, error) {
	var wg sync.WaitGroup
	errCh := make(chan error, count)

	fmt.Println("queing-started")
	for i := 0; i < count; i++ {
		wg.Add(1)
		g.workPool.Submit(func() {
			defer wg.Done()
			id, err := uuid.NewV4()
			if err != nil {
				panic(err)
			}
			data := newEtcdData(id.String())
			errCh <- g.setValue("/data"+id.String(), data)
		})

		if i%1000 == 0 {
			fmt.Printf("queing-progress %d / %d\n", i, count)
		}
	}

	fmt.Println("queing-complete")

	go func() {
		wg.Wait()
		close(errCh)
	}()

	return g.processResults(errCh)
}

func (g *EtcdFiller) setValue(key, data string) error {
	_, err := g.etcdClient.Set("/data/"+key, data, 0)
	return err
}

func (g *EtcdFiller) processResults(errCh chan error) (int, error) {
	var totalResults int
	var errorResults int

	for err := range errCh {
		if err != nil {
			fmt.Printf("failed-seeding-desired-lrps %v\n", err)
			errorResults++
		}
		totalResults++
	}

	errorRate := float64(errorResults) / float64(totalResults)
	if errorRate > ERROR_TOLERANCE {
		err := fmt.Errorf("Error rate of %.3f exceeds tolerance of %.3f", errorRate, ERROR_TOLERANCE)
		fmt.Printf("failed %v\n", err)
		return 0, err
	}

	return totalResults - errorResults, nil
}

func newEtcdData(guid string) string {
	return `{
    "process_guid":"grace-1",
    "domain":"test",
    "rootfs":"docker:///onsi/grace-busybox",
    "instances":1,
    "ports":[
        8080
    ],
    "action":{
        "run":{
            "path":"/grace",
            "args":[
                "-chatty"
            ],
            "dir":"/tmp",
            "user":"root"
        }
    },
    "routes":{
        "cf-router":[
            {
                "hostnames": [
                  "grace.app-domain.com"
                ],
                "port": 8080
            }
        ]
    }
	}`
}
