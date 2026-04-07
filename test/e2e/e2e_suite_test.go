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
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/PalenaAI/langfuse-operator/test/utils"
)

var (
	// projectImage is the operator image built and loaded into the kind cluster.
	// Override via IMG env var (e.g., IMG=my-registry/langfuse-operator:dev).
	projectImage = "ghcr.io/palenaai/langfuse-operator:e2e-test"
)

// operatorNamespace is where the operator itself is deployed.
const operatorNamespace = "langfuse-operator-system"

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting langfuse-operator E2E test suite\n")
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	if img := os.Getenv("IMG"); img != "" {
		projectImage = img
	}

	By("building the operator image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the operator image")

	By("loading the operator image into kind")
	err = utils.LoadImageToKindClusterWithName(projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the operator image into kind")

	By("creating operator namespace")
	cmd = exec.Command("kubectl", "create", "ns", operatorNamespace)
	_, _ = utils.Run(cmd) // ignore if exists

	By("labeling namespace with restricted pod security")
	cmd = exec.Command("kubectl", "label", "--overwrite", "ns", operatorNamespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	By("installing CRDs")
	cmd = exec.Command("make", "install")
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("deploying the operator")
	cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to deploy the operator")
})

var _ = AfterSuite(func() {
	By("undeploying the operator")
	cmd := exec.Command("make", "undeploy")
	_, _ = utils.Run(cmd)

	By("uninstalling CRDs")
	cmd = exec.Command("make", "uninstall")
	_, _ = utils.Run(cmd)

	By("removing operator namespace")
	cmd = exec.Command("kubectl", "delete", "ns", operatorNamespace, "--ignore-not-found")
	_, _ = utils.Run(cmd)
})
