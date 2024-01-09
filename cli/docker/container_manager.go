package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bitrise-io/bitrise/log"
	"github.com/bitrise-io/bitrise/models"
	"github.com/bitrise-io/go-utils/command"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type RunningContainer struct {
	Name string // TODO refactor to use docker sdk, and return container ID instead of name
}

func (rc *RunningContainer) Destroy() error {
	_, err := command.New("docker", "rm", "--force", rc.Name).RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		// rc.logger.Errorf(out)
		return fmt.Errorf("remove docker container: %w", err)
	}
	return nil
}

func (rc *RunningContainer) ExecuteCommandArgs(envs []string) []string {
	args := []string{"exec"}

	for _, env := range envs {
		args = append(args, "-e", env)
	}

	args = append(args, rc.Name)

	return args
}

type ContainerManager struct {
	logger             log.Logger
	workflowContainers map[string]*RunningContainer
	serviceContainers  map[string][]*RunningContainer
	client             *client.Client
}

func NewContainerManager(logger log.Logger) *ContainerManager {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Warnf("Docker client failed to initialize (possibly running on unsupported stack): %s", err)
	}

	return &ContainerManager{
		logger:             logger,
		workflowContainers: make(map[string]*RunningContainer),
		serviceContainers:  make(map[string][]*RunningContainer),
		client:             dockerClient,
	}
}

func (cm *ContainerManager) Login(container models.Container, envs map[string]string) error {
	cm.logger.Infof("Running workflow in docker container: %s", container.Image)
	cm.logger.Debugf("Docker cred: %s", container.Credentials)

	if container.Credentials.Username != "" && container.Credentials.Password != "" {
		cm.logger.Debugf("Logging into docker registry: %s", container.Image)

		password := container.Credentials.Password
		if strings.HasPrefix(password, "$") {
			if value, ok := envs[strings.TrimPrefix(container.Credentials.Password, "$")]; ok {
				password = value
			}
		}

		args := []string{"login", "--username", container.Credentials.Username, "--password", password}

		if container.Credentials.Server != "" {
			args = append(args, container.Credentials.Server)
		} else if len(strings.Split(container.Image, "/")) > 2 {
			args = append(args, container.Image)
		}

		cm.logger.Debugf("Running command: docker %s", strings.Join(args, " "))

		out, err := command.New("docker", args...).RunAndReturnTrimmedCombinedOutput()
		if err != nil {
			cm.logger.Errorf(out)
			return fmt.Errorf("run docker login: %w", err)
		}
	}
	return nil
}

func (cm *ContainerManager) StartWorkflowContainer(container models.Container, workflowID string) (*RunningContainer, error) {
	containerName := fmt.Sprintf("workflow-%s", workflowID)
	dockerMountOverrides := strings.Split(os.Getenv("BITRISE_DOCKER_MOUNT_OVERRIDES"), ",")
	// TODO: make sure the sleep command works across OS flavours
	runningContainer, err := cm.startContainer(container, containerName, dockerMountOverrides, "sleep infinity", "/bitrise/src")
	if err != nil {
		return nil, fmt.Errorf("start workflow container: %w", err)
	}
	cm.workflowContainers[workflowID] = runningContainer
	return runningContainer, nil
}

func (cm *ContainerManager) StartServiceContainer(service models.Container, workflowID string, serviceName string) (*RunningContainer, error) {
	// Naming the container other than the service name, can cause issues with network calls
	runningContainer, err := cm.startContainer(service, serviceName, []string{}, "", "")
	if err != nil {
		return nil, fmt.Errorf("start service container: %w", err)
	}
	cm.serviceContainers[workflowID] = append(cm.serviceContainers[workflowID], runningContainer)
	return runningContainer, nil
}

func (cm *ContainerManager) GetWorkflowContainer(workflowID string) *RunningContainer {
	return cm.workflowContainers[workflowID]
}

func (cm *ContainerManager) GetServiceContainers(workflowID string) []*RunningContainer {
	return cm.serviceContainers[workflowID]
}

func (cm *ContainerManager) DestroyAllContainers() error {
	for _, container := range cm.workflowContainers {
		if err := container.Destroy(); err != nil {
			return fmt.Errorf("destroy workflow container: %w", err)
		}
	}

	for _, containers := range cm.serviceContainers {
		for _, container := range containers {
			if err := container.Destroy(); err != nil {
				return fmt.Errorf("destroy service container: %w", err)
			}
		}
	}

	return nil
}

func (cm *ContainerManager) startContainer(container models.Container,
	name string,
	volumes []string,
	commandArgs, workingDir string,
) (*RunningContainer, error) {
	cm.ensureNetwork()

	dockerRunArgs := []string{"run",
		"--platform", "linux/amd64",
		"--network=bitrise",
		"-d",
	}

	for _, o := range volumes {
		dockerRunArgs = append(dockerRunArgs, "-v", o)
	}

	for _, env := range container.Envs {
		for name, value := range env {
			dockerRunArgs = append(dockerRunArgs, "-e", fmt.Sprintf("%s=%s", name, value))
		}
	}

	for _, port := range container.Ports {
		dockerRunArgs = append(dockerRunArgs, "-p", port)
	}

	if workingDir != "" {
		dockerRunArgs = append(dockerRunArgs, "-w", workingDir)
	}

	if container.Options != "" {
		log.Infof("Container options: %s", container.Options)
		optionsList := strings.Split(container.Options, " ")
		dockerRunArgs = append(dockerRunArgs, optionsList...)
	}

	dockerRunArgs = append(dockerRunArgs,
		fmt.Sprintf("--name=%s", name),
		container.Image,
	)

	// TODO: think about enabling setting this on the public UI as well
	if commandArgs != "" {
		commandArgsList := strings.Split(commandArgs, " ")
		dockerRunArgs = append(dockerRunArgs, commandArgsList...)
	}

	log.Infof("Running command: docker %s", strings.Join(dockerRunArgs, " "))
	out, err := command.New("docker", dockerRunArgs...).RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		log.Errorf(out)
		return nil, fmt.Errorf("run docker container: %w", err)
	}

	if err := cm.healthCheckContainer(err, name); err != nil {
		return nil, fmt.Errorf("container unable to start properly: %w", err)
	}

	runningContainer := &RunningContainer{
		Name: name,
	}
	return runningContainer, nil
}

func (cm *ContainerManager) healthCheckContainer(err error, name string) error {
	containers, err := cm.client.ContainerList(context.Background(), types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return fmt.Errorf("list containers: %w", err)
	}

	if len(containers) != 1 {
		return fmt.Errorf("multiple containers with the same name found: %s", name)
	}

	inspect, err := cm.client.ContainerInspect(context.Background(), containers[0].ID)
	if err != nil {
		return fmt.Errorf("inspect container: %w", err)
	}

	if inspect.State.Health == nil {
		cm.logger.Infof("No healthcheck is defined for container, assuming healthy...")
		return nil
	}

	retries := 0
	for inspect.State.Health.Status != "healthy" {
		if retries > 30 {
			return fmt.Errorf("container unable to start properly: %w", err)
		}

		// TODO: more sophisticated retry logic
		// this solution prefers quick retries at the beginning and constant for the rest
		sleep := 5
		if retries < 5 {
			sleep = retries
		}
		time.Sleep(time.Duration(sleep) * time.Second)

		cm.logger.Infof("Waiting for container (%s) to start...", name)
		inspect, err = cm.client.ContainerInspect(context.Background(), containers[0].ID)
		if err != nil {
			return fmt.Errorf("inspect container: %w", err)
		}
		retries++

	}

	return nil
}

func (cm *ContainerManager) ensureNetwork() {
	// TODO: check if network exists
	out, err := command.New("docker", "network", "create", "bitrise").RunAndReturnTrimmedCombinedOutput()
	if err != nil {
		log.Infof("create network: %s", out)
	}
}
