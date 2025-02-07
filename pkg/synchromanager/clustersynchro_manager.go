package synchromanager

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	clusterv1alpha2 "github.com/clusterpedia-io/api/cluster/v1alpha2"

	crdclientset "github.com/clusterpedia-io/clusterpedia/pkg/generated/clientset/versioned"
	"github.com/clusterpedia-io/clusterpedia/pkg/generated/informers/externalversions"
	clusterlister "github.com/clusterpedia-io/clusterpedia/pkg/generated/listers/cluster/v1alpha2"
	"github.com/clusterpedia-io/clusterpedia/pkg/storage"
	"github.com/clusterpedia-io/clusterpedia/pkg/synchromanager/clustersynchro"
	"github.com/clusterpedia-io/clusterpedia/pkg/synchromanager/features"
	clusterpediafeature "github.com/clusterpedia-io/clusterpedia/pkg/utils/feature"
)

const ClusterSynchroControllerFinalizer = "clusterpedia.io/cluster-synchro-controller"

const maxClusterSynchroRetry = 5

type Manager struct {
	runLock sync.Mutex
	stopCh  <-chan struct{}

	kubeclient         clientset.Interface
	clusterpediaclient crdclientset.Interface
	informerFactory    externalversions.SharedInformerFactory

	queue           workqueue.RateLimitingInterface
	storage         storage.StorageFactory
	clusterlister   clusterlister.PediaClusterLister
	clusterInformer cache.SharedIndexInformer

	synchrolock      sync.RWMutex
	synchros         map[string]*clustersynchro.ClusterSynchro
	synchroWaitGroup wait.Group
}

func NewManager(kubeclient clientset.Interface, client crdclientset.Interface, storage storage.StorageFactory) *Manager {
	factory := externalversions.NewSharedInformerFactory(client, 0)
	clusterinformer := factory.Cluster().V1alpha2().PediaClusters()

	manager := &Manager{
		informerFactory:    factory,
		kubeclient:         kubeclient,
		clusterpediaclient: client,

		storage:         storage,
		clusterlister:   clusterinformer.Lister(),
		clusterInformer: clusterinformer.Informer(),
		queue: workqueue.NewRateLimitingQueue(
			workqueue.NewItemExponentialFailureRateLimiter(2*time.Second, 5*time.Second),
		),

		synchros: make(map[string]*clustersynchro.ClusterSynchro),
	}

	clusterinformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    manager.addCluster,
			UpdateFunc: manager.updateCluster,
			DeleteFunc: manager.deleteCluster,
		},
	)

	return manager
}

func (manager *Manager) Run(workers int, stopCh <-chan struct{}) {
	manager.runLock.Lock()
	defer manager.runLock.Unlock()
	if manager.stopCh != nil {
		klog.Fatal("clustersynchro manager is already running...")
	}
	klog.Info("Start Informer Factory")

	// informerFactory should not be controlled by stopCh
	stopInformer := make(chan struct{})
	manager.informerFactory.Start(stopInformer)
	if !cache.WaitForCacheSync(stopCh, manager.clusterInformer.HasSynced) {
		klog.Fatal("clustersynchro manager: wait for informer factory failed")
	}

	manager.stopCh = stopCh

	klog.InfoS("Start Manager Cluster Worker", "workers", workers)
	var waitGroup sync.WaitGroup
	for i := 0; i < workers; i++ {
		waitGroup.Add(1)

		go func() {
			defer waitGroup.Done()
			wait.Until(manager.worker, time.Second, manager.stopCh)
		}()
	}

	<-manager.stopCh
	klog.Info("receive stop signal, stop...")

	manager.queue.ShutDown()
	waitGroup.Wait()

	klog.Info("wait for cluster synchros stop...")
	manager.synchroWaitGroup.Wait()
	klog.Info("cluster synchro manager stoped.")
}

func (manager *Manager) addCluster(obj interface{}) {
	manager.enqueue(obj)
}

func (manager *Manager) updateCluster(older, newer interface{}) {
	oldObj := older.(*clusterv1alpha2.PediaCluster)
	newObj := newer.(*clusterv1alpha2.PediaCluster)
	if newObj.DeletionTimestamp.IsZero() && equality.Semantic.DeepEqual(oldObj.Spec, newObj.Spec) {
		return
	}

	manager.enqueue(newer)
}

func (manager *Manager) deleteCluster(obj interface{}) {
	manager.enqueue(obj)
}

func (manager *Manager) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	manager.queue.Add(key)
}

func (manager *Manager) worker() {
	for manager.processNextCluster() {
		select {
		case <-manager.stopCh:
			return
		default:
		}
	}
}

func (manager *Manager) processNextCluster() (continued bool) {
	key, shutdown := manager.queue.Get()
	if shutdown {
		return false
	}
	defer manager.queue.Done(key)
	continued = true

	_, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		klog.Error(err)
		return
	}

	klog.InfoS("reconcile cluster", "cluster", name)
	cluster, err := manager.clusterlister.Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.InfoS("cluster has been deleted", "cluster", name)
			return
		}

		klog.ErrorS(err, "Failed to get cluster from cache", "cluster", name)
		return
	}

	cluster = cluster.DeepCopy()
	if err := manager.reconcileCluster(cluster); err != nil {
		klog.ErrorS(err, "Failed to reconcile cluster", "cluster", name, "num requeues", manager.queue.NumRequeues(key))
		if manager.queue.NumRequeues(key) < maxClusterSynchroRetry {
			manager.queue.AddRateLimited(key)
			return
		}
		klog.V(2).Infof("Dropping cluster %q out of the queue: %v", key, err)
	}
	manager.queue.Forget(key)
	return
}

// if err returned is not nil, cluster will be requeued
func (manager *Manager) reconcileCluster(cluster *clusterv1alpha2.PediaCluster) (err error) {
	if !cluster.DeletionTimestamp.IsZero() {
		klog.InfoS("remove cluster", "cluster", cluster.Name)
		if err := manager.removeCluster(cluster.Name); err != nil {
			klog.ErrorS(err, "Failed to remove cluster", cluster.Name)
			return err
		}

		if !controllerutil.ContainsFinalizer(cluster, ClusterSynchroControllerFinalizer) {
			return nil
		}

		// remove finalizer
		controllerutil.RemoveFinalizer(cluster, ClusterSynchroControllerFinalizer)
		if _, err := manager.clusterpediaclient.ClusterV1alpha2().PediaClusters().Update(context.TODO(), cluster, metav1.UpdateOptions{}); err != nil {
			klog.ErrorS(err, "Failed to remove finializer", "cluster", cluster.Name)
			return err
		}
		return nil
	}

	// ensure finalizer
	if !controllerutil.ContainsFinalizer(cluster, ClusterSynchroControllerFinalizer) {
		controllerutil.AddFinalizer(cluster, ClusterSynchroControllerFinalizer)

		if _, err = manager.clusterpediaclient.ClusterV1alpha2().PediaClusters().Update(context.TODO(), cluster, metav1.UpdateOptions{}); err != nil {
			klog.ErrorS(err, "Failed to add finializer", "cluster", cluster.Name)
			return err
		}
	}

	config, err := buildClusterConfig(cluster)
	if err != nil {
		klog.ErrorS(err, "Failed to build cluster config", "cluster", cluster.Name)
		manager.UpdateClusterSynchroCondition(cluster.Name, clusterv1alpha2.InvalidConfigConditionReason, "invalid cluster config: "+err.Error(), metav1.ConditionFalse)
		return nil
	}

	// if `AllowSyncAllResources` is not enabled, then check whether the all-resource wildcard is used
	if !clusterpediafeature.FeatureGate.Enabled(features.AllowSyncAllResources) {
		for _, groupResources := range cluster.Spec.SyncResources {
			if groupResources.Group == "*" {
				// When using the all-resource wildcard without feature gate enabled,
				// it just updates the condition information and
				// does not stop it if cluster synchro is already running.
				//
				// If have better suggestions can be discussed in the https://github.com/clusterpedia-io/clusterpedia/issues.
				manager.UpdateClusterSynchroCondition(cluster.Name, clusterv1alpha2.InvalidSyncResourceConditionReason,
					"ClusterSynchro Manager's feature gate `AllowSyncAllResources` is not enabled, cannot use all-resources wildcard", metav1.ConditionFalse)
				return nil
			}
		}
	}

	manager.synchrolock.RLock()
	synchro := manager.synchros[cluster.Name]
	manager.synchrolock.RUnlock()
	if synchro != nil && !reflect.DeepEqual(synchro.RESTConfig, config) {
		klog.InfoS("cluster config is changed, rebuild cluster synchro", "cluster", cluster.Name)

		synchro.Shutdown(true)
		synchro = nil

		manager.synchrolock.Lock()
		manager.synchros[cluster.Name] = synchro
		manager.synchrolock.Unlock()
	}

	// create resource synchro
	if synchro == nil {
		synchro, err = clustersynchro.New(cluster.Name, config, manager.storage, manager)
		if err != nil {
			manager.UpdateClusterSynchroCondition(cluster.Name, clusterv1alpha2.InitialFailedConditionReason, err.Error(), metav1.ConditionFalse)
			klog.ErrorS(err, "Failed to create cluster synchro", "cluster", cluster.Name)
			// requeue
			return err
		}

		manager.synchroWaitGroup.StartWithChannel(manager.stopCh, synchro.Run)
	}

	manager.UpdateClusterSynchroCondition(cluster.Name, clusterv1alpha2.RunningConditionReason, "", metav1.ConditionTrue)

	synchro.SetResources(cluster.Spec.SyncResources, cluster.Spec.SyncAllCustomResources)

	manager.synchrolock.Lock()
	manager.synchros[cluster.Name] = synchro
	manager.synchrolock.Unlock()
	return nil
}

func (manager *Manager) removeCluster(name string) error {
	manager.synchrolock.Lock()
	synchro := manager.synchros[name]
	delete(manager.synchros, name)
	manager.synchrolock.Unlock()

	if synchro != nil {
		// not update removed cluster,
		// and ensure that no more data is being synchronized to the resource storage
		synchro.Shutdown(false)
	}
	return manager.cleanCluster(name)
}

func (manager *Manager) cleanCluster(name string) error {
	return manager.storage.CleanCluster(context.TODO(), name)
}

func (manager *Manager) UpdateClusterSynchroCondition(name, reason, message string, status metav1.ConditionStatus) {
	synchroCondition := metav1.Condition{
		Type:    clusterv1alpha2.ClusterSynchroCondition,
		Reason:  reason,
		Status:  status,
		Message: message,
	}

	clusterStatus := &clusterv1alpha2.ClusterStatus{Conditions: []metav1.Condition{synchroCondition}}

	if err := manager.UpdateClusterStatus(context.TODO(), name, clusterStatus); err != nil {
		klog.ErrorS(err, "Failed to update cluster synchro condition status", "cluster", name, "condition", synchroCondition)
	}
}

func (manager *Manager) UpdateClusterStatus(ctx context.Context, name string, status *clusterv1alpha2.ClusterStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cluster, err := manager.clusterlister.Get(name)
		if err != nil {
			return err
		}

		lastStatus := cluster.Status
		cluster = cluster.DeepCopy()

		if status.Version != "" {
			cluster.Status.Version = status.Version
		}
		if status.SyncResources != nil {
			cluster.Status.SyncResources = status.SyncResources
		}
		for _, condition := range status.Conditions {
			meta.SetStatusCondition(&cluster.Status.Conditions, condition)
		}

		if equality.Semantic.DeepEqual(cluster.Status, lastStatus) {
			return nil
		}

		_, err = manager.clusterpediaclient.ClusterV1alpha2().PediaClusters().UpdateStatus(ctx, cluster, metav1.UpdateOptions{})
		if err == nil {
			klog.V(2).InfoS("Update Cluster Status", "cluster", cluster.Name, "conditions", status.Conditions)
			return nil
		}
		return err
	})
}

func buildClusterConfig(cluster *clusterv1alpha2.PediaCluster) (*rest.Config, error) {
	if len(cluster.Spec.Kubeconfig) != 0 {
		clientconfig, err := clientcmd.NewClientConfigFromBytes(cluster.Spec.Kubeconfig)
		if err != nil {
			return nil, err
		}
		return clientconfig.ClientConfig()
	}

	if cluster.Spec.APIServer == "" {
		return nil, errors.New("Cluster APIServer Endpoint is required")
	}

	if len(cluster.Spec.TokenData) == 0 &&
		(len(cluster.Spec.CertData) == 0 || len(cluster.Spec.KeyData) == 0) {
		return nil, errors.New("Cluster APIServer's Token or Cert is required")
	}

	config := &rest.Config{
		Host: cluster.Spec.APIServer,
	}

	if len(cluster.Spec.CAData) != 0 {
		config.TLSClientConfig.CAData = cluster.Spec.CAData
	} else {
		config.TLSClientConfig.Insecure = true
	}

	if len(cluster.Spec.CertData) != 0 && len(cluster.Spec.KeyData) != 0 {
		config.TLSClientConfig.CertData = cluster.Spec.CertData
		config.TLSClientConfig.KeyData = cluster.Spec.KeyData
	}

	if len(cluster.Spec.TokenData) != 0 {
		config.BearerToken = string(cluster.Spec.TokenData)
	}
	return config, nil
}
