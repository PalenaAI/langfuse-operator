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

var _ = Describe("Operator", Ordered, func() {
	var controllerPodName string

	AfterEach(func() {
		if CurrentSpecReport().Failed() {
			By("collecting controller logs on failure")
			if controllerPodName != "" {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", operatorNamespace)
				logs, err := utils.Run(cmd)
				if err == nil {
					_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n%s", logs)
				}
			}

			By("collecting events on failure")
			cmd := exec.Command("kubectl", "get", "events", "-n", operatorNamespace,
				"--sort-by=.lastTimestamp")
			events, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Events:\n%s", events)
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)

	It("should have a running controller pod", func() {
		verifyControllerUp := func(g Gomega) {
			cmd := exec.Command("kubectl", "get", "pods",
				"-l", "control-plane=controller-manager",
				"-o", "go-template={{ range .items }}"+
					"{{ if not .metadata.deletionTimestamp }}"+
					"{{ .metadata.name }}"+
					"{{ \"\\n\" }}{{ end }}{{ end }}",
				"-n", operatorNamespace,
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			podNames := utils.GetNonEmptyLines(output)
			g.Expect(podNames).To(HaveLen(1), "expected exactly 1 controller pod")
			controllerPodName = podNames[0]
			g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

			cmd = exec.Command("kubectl", "get", "pods", controllerPodName,
				"-o", "jsonpath={.status.phase}",
				"-n", operatorNamespace,
			)
			phase, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(phase).To(Equal("Running"))
		}
		Eventually(verifyControllerUp).Should(Succeed())
	})

	It("should have CRDs registered", func() {
		cmd := exec.Command("kubectl", "get", "crds")
		output, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(ContainSubstring("langfuseinstances.langfuse.palena.ai"))
		Expect(output).To(ContainSubstring("langfuseorganizations.langfuse.palena.ai"))
		Expect(output).To(ContainSubstring("langfuseprojects.langfuse.palena.ai"))
	})

	It("should pass health checks", func() {
		verifyHealthy := func(g Gomega) {
			// The operator image is distroless (no shell/wget), so we verify
			// health by checking that the pod's readiness probe is passing.
			cmd := exec.Command("kubectl", "get", "pods",
				"-l", "control-plane=controller-manager",
				"-n", operatorNamespace,
				"-o", "jsonpath={.items[0].status.containerStatuses[0].ready}")
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("true"), "controller container should be ready (readiness probe passing)")
		}
		Eventually(verifyHealthy).Should(Succeed())
	})
})
