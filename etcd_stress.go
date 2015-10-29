package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/jenspinney/etcdstress/generator"
)

var (
	etcdFlags             *ETCDFlags
	dataCountRequested    int
	numPopulateWorkers    int
	expectedDataCount     int
	expectedDataTolerance float64

	etcdClient *etcd.Client
)

const (
	DEBUG = "debug"
	INFO  = "info"
	ERROR = "error"
	FATAL = "fatal"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.IntVar(&dataCountRequested, "dataCountRequested", 0, "number of entries to create")
	flag.IntVar(&numPopulateWorkers, "numPopulateWorkers", 2, "number of workers to use when populating etcd")

	etcdFlags = AddETCDFlags(flag.CommandLine)

	flag.Parse()

	fmt.Println("initializing etcd client")
	etcdOptions, err := etcdFlags.Validate()
	if err != nil {
		panic("failed to load etcd flags")
	}

	etcdClient = initializeEtcdClient(etcdOptions)

	purge("/data")

	fmt.Printf("%#v\n\n", flag.Args())
	if dataCountRequested > 0 {
		fmt.Println("Here about to generate")
		dataGenerator := generator.NewEtcdFiller(numPopulateWorkers, etcdClient)
		expectedDataCount, err = dataGenerator.Generate(dataCountRequested)
		if err != nil {
			panic("failed to generate data")
		}
		fmt.Printf("requested %d entries, received %d\n", dataCountRequested, expectedDataCount)
		expectedDataTolerance = float64(expectedDataCount) * generator.ERROR_TOLERANCE
	}
}

type ETCDFlags struct {
	etcdCertFile           string
	etcdKeyFile            string
	etcdCaFile             string
	clusterUrls            string
	clientSessionCacheSize int
	maxIdleConnsPerHost    int
}

func AddETCDFlags(flagSet *flag.FlagSet) *ETCDFlags {
	flags := &ETCDFlags{}

	flagSet.StringVar(
		&flags.clusterUrls,
		"etcdCluster",
		"http://127.0.0.1:4001",
		"comma-separated list of etcd URLs (scheme://ip:port)",
	)
	flagSet.StringVar(
		&flags.etcdCertFile,
		"etcdCertFile",
		"",
		"Location of the client certificate for mutual auth",
	)
	flagSet.StringVar(
		&flags.etcdKeyFile,
		"etcdKeyFile",
		"",
		"Location of the client key for mutual auth",
	)
	flagSet.StringVar(
		&flags.etcdCaFile,
		"etcdCaFile",
		"",
		"Location of the CA certificate for mutual auth",
	)

	flagSet.IntVar(
		&flags.clientSessionCacheSize,
		"etcdSessionCacheSize",
		0,
		"Capacity of the ClientSessionCache option on the TLS configuration. If zero, golang's default will be used",
	)
	flagSet.IntVar(
		&flags.maxIdleConnsPerHost,
		"etcdMaxIdleConnsPerHost",
		0,
		"Controls the maximum number of idle (keep-alive) connctions per host. If zero, golang's default will be used",
	)
	return flags
}

func (flags *ETCDFlags) Validate() (*ETCDOptions, error) {
	scheme := ""
	clusterUrls := strings.Split(flags.clusterUrls, ",")
	for i, uString := range clusterUrls {
		uString = strings.TrimSpace(uString)
		clusterUrls[i] = uString
		u, err := url.Parse(uString)
		if err != nil {
			return nil, fmt.Errorf("Invalid cluster URL: '%s', error: [%s]", uString, err.Error())
		}
		if scheme == "" {
			if u.Scheme != "http" && u.Scheme != "https" {
				return nil, errors.New("Invalid scheme: " + uString)
			}
			scheme = u.Scheme
		} else if scheme != u.Scheme {
			return nil, fmt.Errorf("Multiple url schemes provided: %s", flags.clusterUrls)
		}
	}

	isSSL := false
	if scheme == "https" {
		isSSL = true
		if flags.etcdCertFile == "" {
			return nil, errors.New("Cert file must be provided for https connections")
		}
		if flags.etcdKeyFile == "" {
			return nil, errors.New("Key file must be provided for https connections")
		}
	}

	return &ETCDOptions{
		CertFile:    flags.etcdCertFile,
		KeyFile:     flags.etcdKeyFile,
		CAFile:      flags.etcdCaFile,
		ClusterUrls: clusterUrls,
		IsSSL:       isSSL,
		ClientSessionCacheSize: flags.clientSessionCacheSize,
		MaxIdleConnsPerHost:    flags.maxIdleConnsPerHost,
	}, nil
}

type ETCDOptions struct {
	CertFile               string
	KeyFile                string
	CAFile                 string
	ClusterUrls            []string
	IsSSL                  bool
	ClientSessionCacheSize int
	MaxIdleConnsPerHost    int
}

func initializeEtcdClient(etcdOptions *ETCDOptions) *etcd.Client {
	var etcdClient *etcd.Client
	var tr *http.Transport

	if etcdOptions.IsSSL {
		if etcdOptions.CertFile == "" || etcdOptions.KeyFile == "" {
			panic(errors.New("Require both cert and key path"))
		}

		var err error
		etcdClient, err = etcd.NewTLSClient(etcdOptions.ClusterUrls, etcdOptions.CertFile, etcdOptions.KeyFile, etcdOptions.CAFile)
		if err != nil {
			panic(err)
		}

		tlsCert, err := tls.LoadX509KeyPair(etcdOptions.CertFile, etcdOptions.KeyFile)
		if err != nil {
			panic(err)
		}

		tlsConfig := &tls.Config{
			Certificates:       []tls.Certificate{tlsCert},
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(etcdOptions.ClientSessionCacheSize),
		}
		tr = &http.Transport{
			TLSClientConfig:     tlsConfig,
			Dial:                etcdClient.DefaultDial,
			MaxIdleConnsPerHost: etcdOptions.MaxIdleConnsPerHost,
		}
		etcdClient.SetTransport(tr)
	} else {
		etcdClient = etcd.NewClient(etcdOptions.ClusterUrls)
	}
	etcdClient.SetConsistency(etcd.STRONG_CONSISTENCY)

	return etcdClient
}

func purge(key string) {
	_, err := etcdClient.Delete(key, true)
	if err != nil {
		matches, matchErr := regexp.Match(".*Key not found.*", []byte(err.Error()))
		if matchErr != nil {
			panic(matchErr)
		}
		if !matches {
			fmt.Println("No data to purge")
		}
	}
}
