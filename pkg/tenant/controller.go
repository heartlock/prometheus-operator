// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tenant

import (
	"fmt"
	"time"

	"github.com/coreos/prometheus-operator/pkg/client/auth/v1alpha1"
	"github.com/coreos/prometheus-operator/pkg/k8sutil"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	extensionsobj "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/pkg/util/workqueue"
	"k8s.io/client-go/tools/cache"
)

const (
	tprTenant = "tenant." + v1alpha1.TPRGroup

	resyncPeriod = 5 * time.Minute
)

// TenantController manages lify cycle of Prometheus deployments and
// monitoring configurations.
type TenantController struct {
	kclient *kubernetes.Clientset
	mclient *v1alpha1.AuthV1alpha1Client
	logger  log.Logger

	tenInf cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface

	host   string
	config Config
}

// Config defines configuration parameters for the TenantController.
type Config struct {
	Host       string
	KubeConfig string
}

// New creates a new controller.
func New(conf Config, logger log.Logger) (*TenantController, error) {
	cfg, err := k8sutil.NewClusterConfig(conf.Host, conf.KubeConfig)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	mclient, err := v1alpha1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	c := &TenantController{
		kclient: client,
		mclient: mclient,
		logger:  logger,
		queue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "prometheus"),
		host:    cfg.Host,
		config:  conf,
	}

	c.tenInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  mclient.Tenants(api.NamespaceAll).List,
			WatchFunc: mclient.Tenants(api.NamespaceAll).Watch,
		},
		&v1alpha1.Tenant{}, resyncPeriod, cache.Indexers{},
	)
	c.tenInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAddTenant,
		DeleteFunc: c.handleDeleteTenant,
		UpdateFunc: c.handleUpdateTenant,
	})

	return c, nil
}

// Run the controller.
func (c *TenantController) Run(stopc <-chan struct{}) error {
	defer c.queue.ShutDown()

	errChan := make(chan error)
	go func() {
		v, err := c.kclient.Discovery().ServerVersion()
		if err != nil {
			errChan <- errors.Wrap(err, "communicating with server failed")
			return
		}
		c.logger.Log("msg", "connection established", "cluster-version", v)

		if err := c.createTPRs(); err != nil {
			errChan <- errors.Wrap(err, "creating TPRs failed")
			return
		}
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
		c.logger.Log("msg", "TPR API endpoints ready")
	case <-stopc:
		return nil
	}

	go c.worker()

	go c.tenInf.Run(stopc)

	<-stopc
	return nil
}

func (c *TenantController) keyFunc(obj interface{}) (string, bool) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Log("msg", "creating key failed", "err", err)
		return k, false
	}
	return k, true
}

func (c *TenantController) handleAddTenant(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}

	c.logger.Log("msg", "Prometheus added", "key", key)
	c.enqueue(key)
}

func (c *TenantController) handleDeleteTenant(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}

	c.logger.Log("msg", "Prometheus deleted", "key", key)
	c.enqueue(key)
}

func (c *TenantController) handleUpdateTenant(old, cur interface{}) {
	key, ok := c.keyFunc(cur)
	if !ok {
		return
	}

	c.logger.Log("msg", "Prometheus updated", "key", key)
	c.enqueue(key)
}

func (c *TenantController) getObject(obj interface{}) (apimetav1.Object, bool) {
	ts, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = ts.Obj
	}

	o, err := meta.Accessor(obj)
	if err != nil {
		c.logger.Log("msg", "get object failed", "err", err)
		return nil, false
	}
	return o, true
}

// enqueue adds a key to the queue. If obj is a key already it gets added directly.
// Otherwise, the key is extracted via keyFunc.
func (c *TenantController) enqueue(obj interface{}) {
	if obj == nil {
		return
	}

	key, ok := obj.(string)
	if !ok {
		key, ok = c.keyFunc(obj)
		if !ok {
			return
		}
	}

	c.queue.Add(key)
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *TenantController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *TenantController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(errors.Wrap(err, fmt.Sprintf("Sync %q failed", key)))
	c.queue.AddRateLimited(key)

	return true
}

func (c *TenantController) sync(key string) error {
	obj, exists, err := c.tenInf.GetIndexer().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		//TODO: delete tenant
		return nil
	}

	t := obj.(*v1alpha1.Tenant)
	if t.Spec.UserName == "" {
		//TODO: do nothing
		return nil
	}

	c.logger.Log("msg", "sync prometheus", "key", key)
	//TODO: sync tenant
	return nil
}

func (c *TenantController) createTPRs() error {
	tprs := []*extensionsobj.ThirdPartyResource{
		{
			ObjectMeta: apimetav1.ObjectMeta{
				Name: tprTenant,
			},
			Versions: []extensionsobj.APIVersion{
				{Name: v1alpha1.TPRVersion},
			},
			Description: "Managed Prometheus server",
		},
	}
	tprClient := c.kclient.Extensions().ThirdPartyResources()

	for _, tpr := range tprs {
		if _, err := tprClient.Create(tpr); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		c.logger.Log("msg", "TPR created", "tpr", tpr.Name)
	}

	// We have to wait for the TPRs to be ready. Otherwise the initial watch may fail.
	err := k8sutil.WaitForTPRReady(c.kclient.CoreV1().RESTClient(), v1alpha1.TPRGroup, v1alpha1.TPRVersion, v1alpha1.TPRTenantName)
	if err != nil {
		return err
	}
	return nil
}
