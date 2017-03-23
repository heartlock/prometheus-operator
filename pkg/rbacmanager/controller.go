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

package rbacmanager

import (
	"fmt"
	"time"

	"github.com/coreos/prometheus-operator/pkg/client/auth/v1alpha1"
	"github.com/coreos/prometheus-operator/pkg/k8sutil"
	"github.com/coreos/prometheus-operator/pkg/tenant"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/util/workqueue"
	"k8s.io/client-go/tools/cache"
)

const (
	tprAlertmanager = "alertmanager." + v1alpha1.TPRGroup

	resyncPeriod = 5 * time.Minute
)

// Controller manages lify cycle of Alertmanager deployments and
// monitoring configurations.
type Controller struct {
	kclient *kubernetes.Clientset
	mclient *v1alpha1.AuthV1alpha1Client
	logger  log.Logger

	nsInf cache.SharedIndexInformer

	queue workqueue.RateLimitingInterface
}

// New creates a new controller.
func New(conf tenant.Config, logger log.Logger) (*Controller, error) {
	cfg, err := k8sutil.NewClusterConfig(conf.Host, conf.KubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating cluster config failed")
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating kubernetes client failed")
	}

	mclient, err := v1alpha1.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating monitoring client failed")
	}

	o := &Controller{
		kclient: client,
		mclient: mclient,
		logger:  logger,
		queue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "rbacmanager"),
	}

	o.nsInf = cache.NewSharedIndexInformer(
		cache.NewListWatchFromClient(o.kclient.Core().RESTClient(), "namespaces", api.NamespaceAll, nil),
		&v1.Namespace{}, resyncPeriod, cache.Indexers{},
	)

	o.nsInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    o.handleNamespaceAdd,
		DeleteFunc: o.handleNamespaceDelete,
		UpdateFunc: o.handleNamespaceUpdate,
	})

	return o, nil
}

// Run the controller.
func (c *Controller) Run(stopc <-chan struct{}) error {
	defer c.queue.ShutDown()

	errChan := make(chan error)
	go func() {
		v, err := c.kclient.Discovery().ServerVersion()
		if err != nil {
			errChan <- errors.Wrap(err, "communicating with server failed")
			return
		}
		c.logger.Log("msg", "connection established", "cluster-version", v)
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

	go c.nsInf.Run(stopc)

	<-stopc
	return nil
}

func (c *Controller) keyFunc(obj interface{}) (string, bool) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.Log("msg", "creating key failed", "err", err)
		return k, false
	}
	return k, true
}

// enqueue adds a key to the queue. If obj is a key already it gets added directly.
// Otherwise, the key is extracted via keyFunc.
func (c *Controller) enqueue(obj interface{}) {
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
func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
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

func (c *Controller) handleNamespaceAdd(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}

	c.logger.Log("msg", "Namespace added", "key", key)
	c.enqueue(key)
}

func (c *Controller) handleNamespaceDelete(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}

	c.logger.Log("msg", "Alertmanager deleted", "key", key)
	c.enqueue(key)
}

func (c *Controller) handleNamespaceUpdate(old, cur interface{}) {
	key, ok := c.keyFunc(cur)
	if !ok {
		return
	}

	c.logger.Log("msg", "Alertmanager updated", "key", key)
	c.enqueue(key)
}

func (c *Controller) sync(key string) error {
	obj, exists, err := c.nsInf.GetIndexer().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		//TODO: delete rbac policy related ns
		return nil
	}

	ns := obj.(*v1.Namespace)
	fmt.Println(ns)
	// if tenant not specified do nothing
	if _, ok := ns.Annotations["tenant"]; !ok {
		return nil
	}
	//rbacClient := c.kclient.Rbac()

	c.logger.Log("msg", "sync alertmanager", "key", key)

	//TODO: sync rbac
	return nil
}
