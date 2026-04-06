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

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/PalenaAI/langfuse-operator/test/utils"
)

const (
	testNamespace = "langfuse-e2e"

	// Timeouts for different stages.
	depTimeout      = "3m" // dependencies are lightweight containers
	langfuseTimeout = "8m" // Langfuse needs DB migrations + ClickHouse init
	cleanupTimeout  = "2m" // garbage collection via owner references
	resourceTimeout = "3m" // operator creating k8s resources
	pollingInterval = 2 * time.Second
)

// --- helpers (all scoped to testNamespace) -----------------------------------

// ownerRefUID returns the UID of the owning LangfuseInstance for a given resource.
func ownerRefUID(resource, name string) string {
	cmd := exec.Command("kubectl", "get", resource, name,
		"-n", testNamespace,
		"-o", "jsonpath={.metadata.ownerReferences[0].uid}")
	out, err := utils.Run(cmd)
	if err != nil {
		return ""
	}
	return out
}

// resourceExists returns true if the given resource exists.
func resourceExists(resource, name string) bool {
	cmd := exec.Command("kubectl", "get", resource, name, "-n", testNamespace)
	_, err := utils.Run(cmd)
	return err == nil
}

// getJSONPath runs kubectl get with a jsonpath and returns the result.
func getJSONPath(resource, name, jsonpath string) (string, error) {
	cmd := exec.Command("kubectl", "get", resource, name,
		"-n", testNamespace,
		"-o", fmt.Sprintf("jsonpath=%s", jsonpath))
	return utils.Run(cmd)
}

// labelValue returns the value of a label on a resource.
func labelValue(resource, name, label string) string {
	cmd := exec.Command("kubectl", "get", resource, name,
		"-n", testNamespace,
		"-o", fmt.Sprintf("go-template={{index .metadata.labels %q}}", label))
	out, err := utils.Run(cmd)
	if err != nil {
		return ""
	}
	return out
}

// --- Test scenarios ----------------------------------------------------------

var _ = Describe("LangfuseInstance", Ordered, func() {

	BeforeAll(func() {
		By("creating the test namespace")
		cmd := exec.Command("kubectl", "create", "ns", testNamespace)
		_, _ = utils.Run(cmd) // ignore if exists
	})

	AfterAll(func() {
		By("deleting the test namespace")
		cmd := exec.Command("kubectl", "delete", "ns", testNamespace,
			"--ignore-not-found", "--timeout=60s")
		_, _ = utils.Run(cmd)
	})

	// Collect diagnostics on failure.
	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			By("collecting test namespace events")
			cmd := exec.Command("kubectl", "get", "events", "-n", testNamespace,
				"--sort-by=.lastTimestamp")
			events, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Events in %s:\n%s", testNamespace, events)
			}

			By("collecting pod statuses")
			cmd = exec.Command("kubectl", "get", "pods", "-n", testNamespace, "-o", "wide")
			pods, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Pods:\n%s", pods)
			}

			By("collecting LangfuseInstance status")
			cmd = exec.Command("kubectl", "get", "langfuseinstances", "-n", testNamespace, "-o", "yaml")
			cr, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "LangfuseInstances:\n%s", cr)
			}

			By("collecting operator logs")
			cmd = exec.Command("kubectl", "logs",
				"-l", "control-plane=controller-manager",
				"-n", operatorNamespace,
				"--tail=100")
			logs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Operator logs (last 100 lines):\n%s", logs)
			}
		}
	})

	// ── External dependencies ─────────────────────────────────────────────

	Context("with external dependencies", Ordered, func() {
		const instanceName = "langfuse-ext"
		var instanceUID string

		BeforeAll(func() {
			By("deploying external dependencies (Postgres, ClickHouse, Redis, MinIO)")
			Expect(utils.KubectlApply(utils.FixtureDir("dependencies.yaml"))).To(Succeed())

			By("waiting for PostgreSQL")
			Expect(utils.WaitForDeploymentReady(testNamespace, "postgres", depTimeout)).To(Succeed())
			By("waiting for ClickHouse")
			Expect(utils.WaitForDeploymentReady(testNamespace, "clickhouse", depTimeout)).To(Succeed())
			By("waiting for Redis")
			Expect(utils.WaitForDeploymentReady(testNamespace, "redis", depTimeout)).To(Succeed())
			By("waiting for MinIO")
			Expect(utils.WaitForDeploymentReady(testNamespace, "minio", depTimeout)).To(Succeed())

			By("waiting for MinIO bucket creation job")
			Expect(utils.WaitForJobComplete(testNamespace, "minio-create-bucket", depTimeout)).To(Succeed())

			By("applying LangfuseInstance CR with external deps")
			Expect(utils.KubectlApply(utils.FixtureDir("langfuse-external.yaml"))).To(Succeed())
		})

		// ── Resource creation ──────────────────────────────────────────

		It("should create the web deployment", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("deployment", instanceName+"-web")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should create the worker deployment", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("deployment", instanceName+"-worker")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should create the web service", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("service", instanceName+"-web")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should create network policies", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("networkpolicy", instanceName+"-web-netpol")).To(BeTrue())
				g.Expect(resourceExists("networkpolicy", instanceName+"-worker-netpol")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		// ── Labels ─────────────────────────────────────────────────────

		It("should set correct labels on web deployment", func() {
			Eventually(func(g Gomega) {
				g.Expect(labelValue("deployment", instanceName+"-web",
					"app.kubernetes.io/name")).To(Equal("langfuse"))
				g.Expect(labelValue("deployment", instanceName+"-web",
					"app.kubernetes.io/instance")).To(Equal(instanceName))
				g.Expect(labelValue("deployment", instanceName+"-web",
					"app.kubernetes.io/component")).To(Equal("web"))
				g.Expect(labelValue("deployment", instanceName+"-web",
					"app.kubernetes.io/managed-by")).To(Equal("langfuse-operator"))
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should set correct labels on worker deployment", func() {
			Eventually(func(g Gomega) {
				g.Expect(labelValue("deployment", instanceName+"-worker",
					"app.kubernetes.io/component")).To(Equal("worker"))
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		// ── Owner references ───────────────────────────────────────────

		It("should set owner references on all resources", func() {
			Eventually(func(g Gomega) {
				uid, err := getJSONPath("langfuseinstance", instanceName, "{.metadata.uid}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(uid).NotTo(BeEmpty())
				instanceUID = uid
			}, resourceTimeout, pollingInterval).Should(Succeed())

			Expect(ownerRefUID("deployment", instanceName+"-web")).To(Equal(instanceUID))
			Expect(ownerRefUID("deployment", instanceName+"-worker")).To(Equal(instanceUID))
			Expect(ownerRefUID("service", instanceName+"-web")).To(Equal(instanceUID))
		})

		// ── Langfuse pods become ready ─────────────────────────────────

		It("should have ready web pods", func() {
			Eventually(func(g Gomega) {
				ready, err := getJSONPath("deployment", instanceName+"-web", "{.status.readyReplicas}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal("1"), "web deployment should have 1 ready replica")
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		It("should have ready worker pods", func() {
			Eventually(func(g Gomega) {
				ready, err := getJSONPath("deployment", instanceName+"-worker", "{.status.readyReplicas}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal("1"), "worker deployment should have 1 ready replica")
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		// ── Status ─────────────────────────────────────────────────────

		It("should report Running phase and Ready status", func() {
			Eventually(func(g Gomega) {
				phase, err := getJSONPath("langfuseinstance", instanceName, "{.status.phase}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(phase).To(Equal("Running"))

				ready, err := getJSONPath("langfuseinstance", instanceName, "{.status.ready}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal("true"))
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		It("should report the correct version in status", func() {
			version, err := getJSONPath("langfuseinstance", instanceName, "{.status.version}")
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("3.163.0"))
		})

		// ── Health endpoint ────────────────────────────────────────────

		It("should serve the Langfuse health endpoint", func() {
			Eventually(func(g Gomega) {
				cmd := exec.Command("kubectl", "run", "e2e-health-check",
					"--image=curlimages/curl:latest",
					"--restart=Never",
					"--namespace", testNamespace,
					"--rm", "-i", "--quiet",
					"--", "-sf", "-o", "/dev/null", "-w", "%{http_code}",
					fmt.Sprintf("http://%s-web.%s.svc:3000/api/public/health", instanceName, testNamespace),
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("200"))
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		// ── Update ─────────────────────────────────────────────────────

		It("should update deployments when image tag changes", func() {
			By("patching the LangfuseInstance image tag")
			Expect(utils.KubectlPatch("langfuseinstance", instanceName, testNamespace,
				`{"spec":{"image":{"tag":"3.163.0"}}}`)).To(Succeed())

			By("verifying the web deployment image is updated")
			Eventually(func(g Gomega) {
				image, err := getJSONPath("deployment", instanceName+"-web",
					"{.spec.template.spec.containers[0].image}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(image).To(ContainSubstring("3.163.0"))
			}, resourceTimeout, pollingInterval).Should(Succeed())

			By("verifying the worker deployment image is updated")
			Eventually(func(g Gomega) {
				image, err := getJSONPath("deployment", instanceName+"-worker",
					"{.spec.template.spec.containers[0].image}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(image).To(ContainSubstring("3.163.0"))
			}, resourceTimeout, pollingInterval).Should(Succeed())

			By("verifying the status version is updated")
			Eventually(func(g Gomega) {
				version, err := getJSONPath("langfuseinstance", instanceName, "{.status.version}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(version).To(Equal("3.163.0"))
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		// ── Deletion ───────────────────────────────────────────────────

		It("should clean up all owned resources when CR is deleted", func() {
			By("deleting the LangfuseInstance CR")
			cmd := exec.Command("kubectl", "delete", "langfuseinstance", instanceName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying web deployment is garbage collected")
			Eventually(func() bool {
				return resourceExists("deployment", instanceName+"-web")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())

			By("verifying worker deployment is garbage collected")
			Eventually(func() bool {
				return resourceExists("deployment", instanceName+"-worker")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())

			By("verifying web service is garbage collected")
			Eventually(func() bool {
				return resourceExists("service", instanceName+"-web")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())

			By("verifying network policies are garbage collected")
			Eventually(func() bool {
				return resourceExists("networkpolicy", instanceName+"-web-netpol")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())
		})
	})

	// ── Managed data stores ───────────────────────────────────────────────

	Context("with managed data stores", Ordered, func() {
		const instanceName = "langfuse-mgd"

		BeforeAll(func() {
			By("ensuring dependencies are still running (shared with external test)")
			Expect(utils.WaitForDeploymentReady(testNamespace, "postgres", depTimeout)).To(Succeed())
			Expect(utils.WaitForDeploymentReady(testNamespace, "minio", depTimeout)).To(Succeed())

			By("applying LangfuseInstance CR with managed ClickHouse and Redis")
			Expect(utils.KubectlApply(utils.FixtureDir("langfuse-managed.yaml"))).To(Succeed())
		})

		AfterAll(func() {
			By("deleting the managed LangfuseInstance CR")
			cmd := exec.Command("kubectl", "delete", "langfuseinstance", instanceName,
				"-n", testNamespace, "--ignore-not-found", "--timeout=60s")
			_, _ = utils.Run(cmd)

			By("cleaning up managed secrets")
			Expect(utils.KubectlDelete(utils.FixtureDir("langfuse-managed.yaml"))).To(Succeed())
		})

		// ── Managed ClickHouse resources ───────────────────────────────

		It("should create a ClickHouse StatefulSet", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("statefulset", instanceName+"-clickhouse")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should create a ClickHouse Service", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("service", instanceName+"-clickhouse")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should create a ClickHouse ConfigMap", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("configmap", instanceName+"-clickhouse")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should set correct labels on ClickHouse StatefulSet", func() {
			Eventually(func(g Gomega) {
				g.Expect(labelValue("statefulset", instanceName+"-clickhouse",
					"app.kubernetes.io/component")).To(Equal("clickhouse"))
				g.Expect(labelValue("statefulset", instanceName+"-clickhouse",
					"app.kubernetes.io/managed-by")).To(Equal("langfuse-operator"))
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		// ── Managed Redis resources ────────────────────────────────────

		It("should create a Redis StatefulSet", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("statefulset", instanceName+"-redis")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should create a Redis Service", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("service", instanceName+"-redis")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should set correct labels on Redis StatefulSet", func() {
			Eventually(func(g Gomega) {
				g.Expect(labelValue("statefulset", instanceName+"-redis",
					"app.kubernetes.io/component")).To(Equal("redis"))
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should set owner references on managed resources", func() {
			var instanceUID string
			Eventually(func(g Gomega) {
				uid, err := getJSONPath("langfuseinstance", instanceName, "{.metadata.uid}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(uid).NotTo(BeEmpty())
				instanceUID = uid
			}, resourceTimeout, pollingInterval).Should(Succeed())

			Expect(ownerRefUID("statefulset", instanceName+"-clickhouse")).To(Equal(instanceUID))
			Expect(ownerRefUID("service", instanceName+"-clickhouse")).To(Equal(instanceUID))
			Expect(ownerRefUID("configmap", instanceName+"-clickhouse")).To(Equal(instanceUID))
			Expect(ownerRefUID("statefulset", instanceName+"-redis")).To(Equal(instanceUID))
			Expect(ownerRefUID("service", instanceName+"-redis")).To(Equal(instanceUID))
		})

		// ── Managed ClickHouse pod health ──────────────────────────────

		It("should have a ready ClickHouse pod", func() {
			Eventually(func(g Gomega) {
				ready, err := getJSONPath("statefulset", instanceName+"-clickhouse", "{.status.readyReplicas}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal("1"))
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		It("should have a ready Redis pod", func() {
			Eventually(func(g Gomega) {
				ready, err := getJSONPath("statefulset", instanceName+"-redis", "{.status.readyReplicas}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(Equal("1"))
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		// ── Web + Worker come up with managed deps ─────────────────────

		It("should create web and worker deployments", func() {
			Eventually(func(g Gomega) {
				g.Expect(resourceExists("deployment", instanceName+"-web")).To(BeTrue())
				g.Expect(resourceExists("deployment", instanceName+"-worker")).To(BeTrue())
			}, resourceTimeout, pollingInterval).Should(Succeed())
		})

		It("should reach Running phase with managed data stores", func() {
			Eventually(func(g Gomega) {
				phase, err := getJSONPath("langfuseinstance", instanceName, "{.status.phase}")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(phase).To(Equal("Running"))
			}, langfuseTimeout, pollingInterval).Should(Succeed())
		})

		// ── Managed deletion cleanup ───────────────────────────────────

		It("should clean up managed resources on deletion", func() {
			By("deleting the LangfuseInstance CR")
			cmd := exec.Command("kubectl", "delete", "langfuseinstance", instanceName,
				"-n", testNamespace, "--timeout=60s")
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying ClickHouse StatefulSet is garbage collected")
			Eventually(func() bool {
				return resourceExists("statefulset", instanceName+"-clickhouse")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())

			By("verifying Redis StatefulSet is garbage collected")
			Eventually(func() bool {
				return resourceExists("statefulset", instanceName+"-redis")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())

			By("verifying web deployment is garbage collected")
			Eventually(func() bool {
				return resourceExists("deployment", instanceName+"-web")
			}, cleanupTimeout, pollingInterval).Should(BeFalse())
		})
	})
})
