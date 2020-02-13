package config

import (
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	io "github.com/elastic/metricbeat-tests-poc/cli/internal"

	packr "github.com/gobuffalo/packr/v2"
	log "github.com/sirupsen/logrus"
)

// opComposeBox the tool's static files where we will embed default Docker compose
// files representing the services and the stacks
var opComposeBox *packr.Box

// Op the tool's configuration, read from tool's workspace
var Op *OpConfig

// OpConfig tool configuration
type OpConfig struct {
	Services  map[string]Service `mapstructure:"services"`
	Stacks    map[string]Stack   `mapstructure:"stacks"`
	Workspace string             `mapstructure:"workspace"`
}

// Service represents the configuration for a service
type Service struct {
	Name string `mapstructure:"Name"`
	Path string `mapstructure:"Path"`
}

// Stack represents the configuration for a stack, which is an aggregation of services
// represented by a Docker Compose
type Stack struct {
	Name string `mapstructure:"Name"`
	Path string `mapstructure:"Path"`
}

// AvailableServices return the services in the configuration file
func AvailableServices() map[string]Service {
	return Op.Services
}

// AvailableStacks return the stacks in the configuration file
func AvailableStacks() map[string]Stack {
	return Op.Stacks
}

// GetComposeFile returns the path of the compose file, looking up the
// tool's workdir or in the static resources already packaged in the binary
func GetComposeFile(isStack bool, composeName string) (string, error) {
	composeFileName := "docker-compose.yml"
	serviceType := "services"
	if isStack {
		serviceType = "stacks"
	}

	composeFilePath := path.Join(Op.Workspace, "compose", serviceType, composeName, composeFileName)
	found, err := io.Exists(composeFilePath)
	if found && err == nil {
		log.WithFields(log.Fields{
			"composeFilePath": composeFilePath,
			"type":            serviceType,
		}).Debug("Compose file found at workdir")

		return composeFilePath, nil
	}

	log.WithFields(log.Fields{
		"composeFilePath": composeFilePath,
		"type":            serviceType,
	}).Debug("Compose file not found at workdir. Extracting from binary resources")

	composeBytes, err := opComposeBox.Find(path.Join(serviceType, composeName, composeFileName))
	if err != nil {
		log.WithFields(log.Fields{
			"composeFileName": composeFileName,
			"error":           err,
			"isStack":         isStack,
			"type":            serviceType,
		}).Error("Could not find compose file.")

		return "", err
	}

	io.MkdirAll(composeFilePath)

	err = ioutil.WriteFile(composeFilePath, composeBytes, 0755)
	if err != nil {
		log.WithFields(log.Fields{
			"composeFilePath": composeFilePath,
			"error":           err,
			"isStack":         isStack,
			"type":            serviceType,
		}).Error("Cannot write file at workdir.")

		return composeFilePath, err
	}

	log.WithFields(log.Fields{
		"composeFilePath": composeFilePath,
		"isStack":         isStack,
		"type":            serviceType,
	}).Debug("Compose file generated at workdir.")

	return composeFilePath, nil
}

// GetServiceConfig configuration of a service
func GetServiceConfig(service string) (Service, bool) {
	return Op.GetServiceConfig(service)
}

// GetServiceConfig configuration of a service
func (c *OpConfig) GetServiceConfig(service string) (Service, bool) {
	srv, exists := c.Services[service]

	return srv, exists
}

// Init creates this tool workspace under user's home, in a hidden directory named ".op"
func Init() {
	configureLogger()

	checkInstalledSoftware()

	InitConfig()
}

// InitConfig initialises configuration
func InitConfig() {
	if Op != nil {
		return
	}

	usr, _ := user.Current()

	w := filepath.Join(usr.HomeDir, ".op")

	newConfig(w)
}

// PutServiceEnvironment puts the environment variables for the service, replacing "SERVICE_"
// with service name in uppercase. The variables are:
//  - SERVICE_VARIANT: where SERVICE is the name of the service (i.e. APACHE_VARIANT)
//  - SERVICE_VERSION: where it represents the version of the service (i.e. APACHE_VERSION)
//  - SERVICE_PATH: where it represents the path to its compose file (i.e. APACHE_PATH)
func PutServiceEnvironment(env map[string]string, service string, serviceVersion string) map[string]string {
	serviceUpper := strings.ToUpper(service)

	env[serviceUpper+"_VARIANT"] = service
	env[serviceUpper+"_VERSION"] = serviceVersion

	srv, exists := Op.Services[service]
	if !exists {
		log.WithFields(log.Fields{
			"service": service,
		}).Warn("Could not find compose file")
	} else {
		env[serviceUpper+"_PATH"] = filepath.Dir(srv.Path)
	}

	return env
}

func checkConfigDirectory(dir string) {
	found, err := io.Exists(dir)
	if found && err == nil {
		return
	}
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"path":  dir,
		}).Fatal("Cannot create directory")
	}
}

func checkConfigDirs(workspace string) {
	servicesPath := path.Join(workspace, "compose", "services")
	stacksPath := path.Join(workspace, "compose", "stacks")

	checkConfigDirectory(servicesPath)
	checkConfigDirectory(stacksPath)

	log.WithFields(log.Fields{
		"servicesPath": servicesPath,
		"stacksPath":   stacksPath,
	}).Debug("'op' workdirs created.")
}

// checkInstalledSoftware checks that the required software is present
func checkInstalledSoftware() {
	log.Debug("Validating required tools...")
	binaries := []string{
		"docker",
		"docker-compose",
	}

	for _, binary := range binaries {
		which(binary)
	}
}

func configureLogger() {
	includeTimestamp := os.Getenv("OP_LOG_INCLUDE_TIMESTAMP")
	fullTimestamp := (strings.ToUpper(includeTimestamp) == "TRUE")

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: fullTimestamp,
	})

	switch logLevel := os.Getenv("OP_LOG_LEVEL"); logLevel {
	case "TRACE":
		log.SetLevel(log.TraceLevel)
	case "DEBUG":
		log.SetLevel(log.DebugLevel)
	case "WARNING":
		log.SetLevel(log.WarnLevel)
	case "ERROR":
		log.SetLevel(log.ErrorLevel)
	case "FATAL":
		log.SetLevel(log.FatalLevel)
	case "PANIC":
		log.SetLevel(log.PanicLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

// newConfig returns a new configuration
func newConfig(workspace string) {
	if Op != nil {
		return
	}

	checkConfigDirs(workspace)

	opConfig := OpConfig{
		Services:  map[string]Service{},
		Stacks:    map[string]Stack{},
		Workspace: workspace,
	}
	Op = &opConfig

	box := packComposeFiles(Op)
	if box == nil {
		log.WithFields(log.Fields{
			"workspace": workspace,
		}).Error("Could not get packaged compose files")
		return
	}

	// add file system services and stacks
	readFilesFromFileSystem("services")
	readFilesFromFileSystem("stacks")

	opComposeBox = box
}

func packComposeFiles(op *OpConfig) *packr.Box {
	box := packr.New("Compose Files", "./compose")

	err := box.Walk(func(boxedPath string, f packr.File) error {
		// there must be three tokens: i.e. 'services/aerospike/docker-compose.yml'
		tokens := strings.Split(boxedPath, string(os.PathSeparator))
		composeType := tokens[0]
		composeName := tokens[1]

		log.WithFields(log.Fields{
			"service": composeName,
			"path":    boxedPath,
		}).Debug("Boxed file")

		if composeType == "stacks" {
			op.Stacks[composeName] = Stack{
				Name: composeName,
				Path: boxedPath,
			}
		} else if composeType == "services" {
			op.Services[composeName] = Service{
				Name: composeName,
				Path: boxedPath,
			}
		}

		return nil
	})
	if err != nil {
		return nil
	}

	return box
}

// reads the docker-compose in the workspace, merging them with what it's
// already boxed in the binary
func readFilesFromFileSystem(serviceType string) {
	basePath := path.Join(Op.Workspace, "compose", serviceType)
	files, err := ioutil.ReadDir(basePath)
	if err != nil {
		log.WithFields(log.Fields{
			"path": basePath,
			"type": serviceType,
		}).Warn("Could not load file system")
		return
	}

	for _, f := range files {
		if f.IsDir() {
			name := f.Name()
			composeFilePath := filepath.Join(basePath, name, "docker-compose.yml")
			found, err := io.Exists(composeFilePath)
			if found && err == nil {
				log.WithFields(log.Fields{
					"service": name,
					"path":    composeFilePath,
				}).Debug("Workspace file")

				if serviceType == "services" {
					// add a service or a stack
					Op.Services[name] = Service{
						Name: f.Name(),
						Path: composeFilePath,
					}
				} else if serviceType == "stacks" {
					Op.Stacks[name] = Stack{
						Name: f.Name(),
						Path: composeFilePath,
					}
				}
			}
		}
	}
}

// which checks if software is installed
func which(software string) {
	path, err := exec.LookPath(software)
	if err != nil {
		log.WithFields(log.Fields{
			"error":    err,
			"software": software,
		}).Fatal("Required binary is not present")
	}

	log.WithFields(log.Fields{
		"software": software,
		"path":     path,
	}).Debug("Binary is present")
}
