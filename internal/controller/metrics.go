/*
Copyright 2026.

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

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "langfuse_operator_reconcile_total",
			Help: "Total number of reconciliations by controller and result",
		},
		[]string{"controller", "result"},
	)
	reconcileErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "langfuse_operator_reconcile_errors_total",
			Help: "Total number of reconciliation errors by controller",
		},
		[]string{"controller"},
	)
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "langfuse_operator_reconcile_duration_seconds",
			Help:    "Duration of reconciliation in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"controller"},
	)
	managedInstances = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "langfuse_operator_managed_instances",
		Help: "Number of managed LangfuseInstance CRs",
	})
)

func init() {
	metrics.Registry.MustRegister(
		reconcileTotal, reconcileErrors, reconcileDuration, managedInstances,
	)
}
