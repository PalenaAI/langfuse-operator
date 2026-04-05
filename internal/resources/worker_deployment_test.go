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

package resources

import (
	"testing"
)

func TestBuildWorkerDeployment_Minimal(t *testing.T) {
	instance := minimalInstance()
	config := buildConfig(instance)
	deploy := BuildWorkerDeployment(instance, config)

	if deploy.Name != "test-worker" {
		t.Errorf("name = %q, want %q", deploy.Name, "test-worker")
	}
	if deploy.Namespace != instance.Namespace {
		t.Errorf("namespace = %q, want %q", deploy.Namespace, instance.Namespace)
	}
	if *deploy.Spec.Replicas != 1 {
		t.Errorf("replicas = %d, want %d", *deploy.Spec.Replicas, 1)
	}

	// Labels
	if deploy.Labels["app.kubernetes.io/component"] != "worker" {
		t.Errorf("component label = %q, want %q", deploy.Labels["app.kubernetes.io/component"], "worker")
	}

	// Container
	containers := deploy.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(containers))
	}
	c := containers[0]
	if c.Image != "langfuse/langfuse:3" {
		t.Errorf("image = %q, want %q", c.Image, "langfuse/langfuse:3")
	}

	// No HTTP port
	if len(c.Ports) != 0 {
		t.Errorf("worker should not expose ports, got %d", len(c.Ports))
	}

	// Liveness probe should be exec-based
	if c.LivenessProbe == nil || c.LivenessProbe.Exec == nil {
		t.Error("liveness probe should be exec-based")
	} else {
		if len(c.LivenessProbe.Exec.Command) < 2 || c.LivenessProbe.Exec.Command[0] != "node" {
			t.Errorf("liveness probe command = %v, want node exec", c.LivenessProbe.Exec.Command)
		}
	}

	// Env: should contain LANGFUSE_WORKER_ENABLED=true
	foundWorkerEnabled := false
	foundConcurrency := false
	for _, e := range c.Env {
		if e.Name == "LANGFUSE_WORKER_ENABLED" {
			if e.Value != "true" {
				t.Errorf("LANGFUSE_WORKER_ENABLED = %q, want %q", e.Value, "true")
			}
			foundWorkerEnabled = true
		}
		if e.Name == "LANGFUSE_WORKER_CONCURRENCY" {
			if e.Value != "10" {
				t.Errorf("LANGFUSE_WORKER_CONCURRENCY = %q, want %q", e.Value, "10")
			}
			foundConcurrency = true
		}
	}
	if !foundWorkerEnabled {
		t.Error("LANGFUSE_WORKER_ENABLED not found in env")
	}
	if !foundConcurrency {
		t.Error("LANGFUSE_WORKER_CONCURRENCY not found in env")
	}
}

func TestBuildWorkerDeployment_CustomReplicas(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Worker.Replicas = ptrInt32(5)
	config := buildConfig(instance)
	deploy := BuildWorkerDeployment(instance, config)

	if *deploy.Spec.Replicas != 5 {
		t.Errorf("replicas = %d, want %d", *deploy.Spec.Replicas, 5)
	}
}

func TestBuildWorkerDeployment_CustomConcurrency(t *testing.T) {
	instance := minimalInstance()
	instance.Spec.Worker.Concurrency = ptrInt32(25)
	config := buildConfig(instance)
	deploy := BuildWorkerDeployment(instance, config)

	c := deploy.Spec.Template.Spec.Containers[0]
	found := false
	for _, e := range c.Env {
		if e.Name == "LANGFUSE_WORKER_CONCURRENCY" {
			if e.Value != "25" {
				t.Errorf("LANGFUSE_WORKER_CONCURRENCY = %q, want %q", e.Value, "25")
			}
			found = true
		}
	}
	if !found {
		t.Error("LANGFUSE_WORKER_CONCURRENCY not found in env")
	}
}
