// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package installer

import (
	"fmt"

	"github.com/elastic/e2e-testing/internal/common"
	"github.com/elastic/e2e-testing/internal/deploy"
	"github.com/elastic/e2e-testing/internal/kibana"
	"github.com/elastic/e2e-testing/internal/utils"
	log "github.com/sirupsen/logrus"
)

// elasticAgentTARPackage implements operations for a RPM installer
type elasticAgentTARPackage struct {
	service deploy.ServiceRequest
	deploy  deploy.Deployment
}

// AttachElasticAgentTARPackage creates an instance for the RPM installer
func AttachElasticAgentTARPackage(deploy deploy.Deployment, service deploy.ServiceRequest) deploy.ServiceOperator {
	return &elasticAgentTARPackage{
		service: service,
		deploy:  deploy,
	}
}

// AddFiles will add files into the service environment, default destination is /
func (i *elasticAgentTARPackage) AddFiles(files []string) error {
	return i.deploy.AddFiles(i.service, files)
}

// Inspect returns info on package
func (i *elasticAgentTARPackage) Inspect() (deploy.ServiceOperatorManifest, error) {
	return deploy.ServiceOperatorManifest{
		WorkDir:    "/opt/Elastic/Agent",
		CommitFile: "/elastic-agent/.elastic-agent.active.commit",
	}, nil
}

// Install installs a TAR package
func (i *elasticAgentTARPackage) Install() error {
	log.Trace("No TAR install instructions")
	return nil
}

// Exec will execute a command within the service environment
func (i *elasticAgentTARPackage) Exec(args []string) (string, error) {
	output, err := i.deploy.ExecIn(i.service, args)
	return output, err
}

// Enroll will enroll the agent into fleet
func (i *elasticAgentTARPackage) Enroll(token string) error {

	cfg, _ := kibana.NewFleetConfig(token)
	args := []string{"/elastic-agent/elastic-agent", "install"}
	for _, arg := range cfg.Flags() {
		args = append(args, arg)
	}

	_, err := i.Exec(args)
	if err != nil {
		return fmt.Errorf("Failed to install the agent with subcommand: %v", err)
	}
	return nil
}

// InstallCerts installs the certificates for a TAR package, using the right OS package manager
func (i *elasticAgentTARPackage) InstallCerts() error {
	return nil
}

// Logs prints logs of service
func (i *elasticAgentTARPackage) Logs() error {
	return i.deploy.Logs(i.service)
}

// Postinstall executes operations after installing a TAR package
func (i *elasticAgentTARPackage) Postinstall() error {
	return nil
}

// Preinstall executes operations before installing a TAR package
func (i *elasticAgentTARPackage) Preinstall() error {
	artifact := "elastic-agent"
	os := "linux"
	arch := "x86_64"
	if utils.GetArchitecture() == "arm64" {
		arch = "arm64"
	}
	extension := "tar.gz"

	binaryName := utils.BuildArtifactName(artifact, common.BeatVersion, common.BeatVersionBase, os, arch, extension, false)
	binaryPath, err := utils.FetchBeatsBinary(binaryName, artifact, common.BeatVersion, common.BeatVersionBase, utils.TimeoutFactor, true)
	if err != nil {
		log.WithFields(log.Fields{
			"artifact":  artifact,
			"version":   common.BeatVersion,
			"os":        os,
			"arch":      arch,
			"extension": extension,
			"error":     err,
		}).Error("Could not download the binary for the agent")
		return err
	}

	err = i.AddFiles([]string{binaryPath})
	if err != nil {
		return err
	}

	output, _ := i.Exec([]string{"mv", fmt.Sprintf("/%s-%s-%s-%s", artifact, common.BeatVersion, os, arch), "/elastic-agent"})
	log.WithField("output", output).Trace("Moved elastic-agent")
	return nil
}

// Start will start a service
func (i *elasticAgentTARPackage) Start() error {
	_, err := i.Exec([]string{"systemctl", "start", "elastic-agent"})
	if err != nil {
		return err
	}
	return nil
}

// Stop will start a service
func (i *elasticAgentTARPackage) Stop() error {
	_, err := i.Exec([]string{"systemctl", "stop", "elastic-agent"})
	if err != nil {
		return err
	}
	return nil
}

// Uninstall uninstalls a TAR package
func (i *elasticAgentTARPackage) Uninstall() error {
	args := []string{"elastic-agent", "uninstall", "-f"}
	_, err := i.Exec(args)
	if err != nil {
		return fmt.Errorf("Failed to uninstall the agent with subcommand: %v", err)
	}
	return nil
}