/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	time "time"

	kyvernov1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	internalinterfaces "github.com/kyverno/kyverno/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// PolicyViolationInformer provides access to a shared informer and lister for
// PolicyViolations.
type PolicyViolationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.PolicyViolationLister
}

type policyViolationInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewPolicyViolationInformer constructs a new informer for PolicyViolation type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewPolicyViolationInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredPolicyViolationInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredPolicyViolationInformer constructs a new informer for PolicyViolation type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredPolicyViolationInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV1().PolicyViolations(namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KyvernoV1().PolicyViolations(namespace).Watch(options)
			},
		},
		&kyvernov1.PolicyViolation{},
		resyncPeriod,
		indexers,
	)
}

func (f *policyViolationInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredPolicyViolationInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *policyViolationInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&kyvernov1.PolicyViolation{}, f.defaultInformer)
}

func (f *policyViolationInformer) Lister() v1.PolicyViolationLister {
	return v1.NewPolicyViolationLister(f.Informer().GetIndexer())
}
