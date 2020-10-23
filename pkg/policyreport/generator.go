package policyreport

import (
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	policyreportclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	reportrequest "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha1"
	policyreportinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha1"
	policyreport "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/constant"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/policystatus"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

// Generator creates report request
type Generator struct {
	dclient                *dclient.Client
	reportRequestInterface reportrequest.PolicyV1alpha1Interface

	reportRequestLister        policyreport.ReportRequestLister
	clusterReportRequestLister policyreport.ClusterReportRequestLister

	// returns true if the cluster report request store has been synced at least once
	reportReqSynced cache.InformerSynced
	// returns true if the namespaced report request store has been synced at at least once
	clusterReportReqSynced cache.InformerSynced

	queue     workqueue.RateLimitingInterface
	dataStore *dataStore

	log logr.Logger
}

// NewReportRequestGenerator returns a new instance of report request generator
func NewReportRequestGenerator(client *policyreportclient.Clientset,
	dclient *dclient.Client,
	reportReqInformer policyreportinformer.ReportRequestInformer,
	clusterReportReqInformer policyreportinformer.ClusterReportRequestInformer,
	policyStatus policystatus.Listener,
	log logr.Logger) *Generator {
	gen := Generator{
		reportRequestInterface:     client.PolicyV1alpha1(),
		dclient:                    dclient,
		clusterReportRequestLister: clusterReportReqInformer.Lister(),
		clusterReportReqSynced:     clusterReportReqInformer.Informer().HasSynced,
		reportRequestLister:        reportReqInformer.Lister(),
		reportReqSynced:            reportReqInformer.Informer().HasSynced,
		queue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:                  newDataStore(),
		log:                        log,
	}

	return &gen
}

//NewDataStore returns an instance of data store
func newDataStore() *dataStore {
	ds := dataStore{
		data: make(map[string]Info),
	}
	return &ds
}

type dataStore struct {
	data map[string]Info
	mu   sync.RWMutex
}

func (ds *dataStore) add(keyHash string, info Info) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	// queue the key hash
	ds.data[keyHash] = info
}

func (ds *dataStore) lookup(keyHash string) Info {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.data[keyHash]
}

func (ds *dataStore) delete(keyHash string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.data, keyHash)
}

//Info is a request to create PV
type Info struct {
	PolicyName string
	Resource   unstructured.Unstructured
	Rules      []kyverno.ViolatedRule
	FromSync   bool
}

func (i Info) toKey() string {
	keys := []string{
		i.PolicyName,
		i.Resource.GetKind(),
		i.Resource.GetNamespace(),
		i.Resource.GetName(),
		strconv.Itoa(len(i.Rules)),
	}
	return strings.Join(keys, "/")
}

type PVEvent struct {
	Namespace map[string][]Info
	Cluster   map[string][]Info
}

//GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
}

func (gen *Generator) enqueue(info Info) {
	keyHash := info.toKey()
	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
}

//Add queues a policy violation create request
func (gen *Generator) Add(infos ...Info) {
	for _, info := range infos {
		gen.enqueue(info)
	}
}

// Run starts the workers
func (gen *Generator) Run(workers int, stopCh <-chan struct{}) {
	logger := gen.log
	defer utilruntime.HandleCrash()
	logger.Info("start")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, gen.reportReqSynced, gen.clusterReportReqSynced) {
		logger.Info("failed to sync informer cache")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(gen.runWorker, constant.PolicyViolationControllerResync, stopCh)
	}

	<-stopCh
}

func (gen *Generator) runWorker() {
	for gen.processNextWorkItem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	logger := gen.log
	if err == nil {
		gen.queue.Forget(key)
		return
	}

	// retires requests if there is error
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		logger.Error(err, "failed to sync report request", "key", key)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}
	gen.queue.Forget(key)
	// remove from data store
	if keyHash, ok := key.(string); ok {
		gen.dataStore.delete(keyHash)
	}
	logger.Error(err, "dropping key out of the queue", "key", key)
}

func (gen *Generator) processNextWorkItem() bool {
	logger := gen.log
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer gen.queue.Done(obj)
		var keyHash string
		var ok bool

		if keyHash, ok = obj.(string); !ok {
			gen.queue.Forget(obj)
			logger.Info("incorrect type; expecting type 'string'", "obj", obj)
			return nil
		}

		// lookup data store
		info := gen.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			gen.queue.Forget(obj)
			logger.Info("empty key")
			return nil
		}

		err := gen.syncHandler(info)
		gen.handleErr(err, obj)
		return nil
	}(obj)

	if err != nil {
		logger.Error(err, "failed to process item")
	}

	return true
}

func (gen *Generator) syncHandler(info Info) error {

	return nil
}