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

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" //nolint:revive,staticcheck
)

const (
	certmanagerVersion = "v1.16.3"
	certmanagerURLTmpl = "https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml"
)

func warnError(err error) {
	_, _ = fmt.Fprintf(GinkgoWriter, "warning: %v\n", err)
}

// Run executes the provided command within the project directory.
func Run(cmd *exec.Cmd) (string, error) {
	dir, _ := GetProjectDir()
	cmd.Dir = dir

	if err := os.Chdir(cmd.Dir); err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "chdir dir: %q\n", err)
	}

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// UninstallCertManager uninstalls the cert manager
func UninstallCertManager() {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "delete", "-f", url)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}
}

// InstallCertManager installs the cert manager bundle.
func InstallCertManager() error {
	url := fmt.Sprintf(certmanagerURLTmpl, certmanagerVersion)
	cmd := exec.Command("kubectl", "apply", "-f", url)
	if _, err := Run(cmd); err != nil {
		return err
	}
	cmd = exec.Command("kubectl", "wait", "deployment.apps/cert-manager-webhook",
		"--for", "condition=Available",
		"--namespace", "cert-manager",
		"--timeout", "5m",
	)

	_, err := Run(cmd)
	return err
}

// IsCertManagerCRDsInstalled checks if any Cert Manager CRDs are installed.
func IsCertManagerCRDsInstalled() bool {
	certManagerCRDs := []string{
		"certificates.cert-manager.io",
		"issuers.cert-manager.io",
	}
	cmd := exec.Command("kubectl", "get", "crds")
	output, err := Run(cmd)
	if err != nil {
		return false
	}
	for _, crd := range certManagerCRDs {
		if strings.Contains(output, crd) {
			return true
		}
	}
	return false
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster.
func LoadImageToKindClusterWithName(name string) error {
	cluster := "kind"
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	cmd := exec.Command("kind", kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines splits output into non-empty lines.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.Split(output, "\n")
	for _, element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}
	return res
}

// GetProjectDir returns the project root directory.
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}
	wd = strings.ReplaceAll(wd, "/test/e2e", "")
	return wd, nil
}

// FixtureDir returns the absolute path to a fixture file.
func FixtureDir(name string) string {
	dir, _ := GetProjectDir()
	return filepath.Join(dir, "test", "e2e", "fixtures", name)
}

// WaitForDeploymentReady waits until a Deployment in the given namespace has all replicas available.
func WaitForDeploymentReady(namespace, name, timeout string) error {
	cmd := exec.Command("kubectl", "wait", "deployment/"+name,
		"--for", "condition=Available",
		"--namespace", namespace,
		"--timeout", timeout,
	)
	_, err := Run(cmd)
	return err
}

// WaitForJobComplete waits for a Job to reach the Complete condition.
func WaitForJobComplete(namespace, name, timeout string) error {
	cmd := exec.Command("kubectl", "wait", "job/"+name,
		"--for", "condition=Complete",
		"--namespace", namespace,
		"--timeout", timeout,
	)
	_, err := Run(cmd)
	return err
}

// KubectlApply applies a YAML file via kubectl.
func KubectlApply(file string) error {
	cmd := exec.Command("kubectl", "apply", "-f", file)
	_, err := Run(cmd)
	return err
}

// KubectlDelete deletes resources defined in a YAML file.
func KubectlDelete(file string) error {
	cmd := exec.Command("kubectl", "delete", "--ignore-not-found", "-f", file)
	_, err := Run(cmd)
	return err
}

// KubectlGet runs kubectl get and returns the output.
func KubectlGet(args ...string) (string, error) {
	cmdArgs := append([]string{"get"}, args...)
	cmd := exec.Command("kubectl", cmdArgs...)
	return Run(cmd)
}

// KubectlGetJSON runs kubectl get with -o json and returns the raw output.
func KubectlGetJSON(args ...string) (string, error) {
	cmdArgs := append([]string{"get"}, args...)
	cmdArgs = append(cmdArgs, "-o", "json")
	cmd := exec.Command("kubectl", cmdArgs...)
	return Run(cmd)
}

// KubectlPatch patches a resource using a strategic merge patch.
func KubectlPatch(resource, name, namespace, patch string) error {
	cmd := exec.Command("kubectl", "patch", resource, name,
		"--namespace", namespace,
		"--type", "merge",
		"-p", patch,
	)
	_, err := Run(cmd)
	return err
}
