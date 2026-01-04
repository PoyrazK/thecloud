package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
)

const (
	NginxImage = "nginx:alpine"
)

type LBProxyAdapter struct {
	cli          *client.Client
	instanceRepo ports.InstanceRepository
	vpcRepo      ports.VpcRepository
}

func NewLBProxyAdapter(instanceRepo ports.InstanceRepository, vpcRepo ports.VpcRepository) (*LBProxyAdapter, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &LBProxyAdapter{
		cli:          cli,
		instanceRepo: instanceRepo,
		vpcRepo:      vpcRepo,
	}, nil
}

func (a *LBProxyAdapter) DeployProxy(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) (string, error) {
	// 1. Ensure image
	reader, err := a.cli.ImagePull(ctx, NginxImage, image.PullOptions{})
	if err != nil {
		return "", err
	}
	_, _ = io.Copy(io.Discard, reader)
	_ = reader.Close()

	// 2. Generate config
	config, err := a.generateNginxConfig(ctx, lb, targets)
	if err != nil {
		return "", err
	}

	// 3. Create temp config file to mount or use env?
	// Docker doesn't support mounting strings directly easily without a file.
	// We'll create a temp directory for LB configs.
	configPath := filepath.Join("/tmp", "thecloud", "lb", lb.ID.String())
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(configPath, "nginx.conf"), []byte(config), 0644); err != nil {
		return "", err
	}

	// 4. Create container
	containerName := fmt.Sprintf("lb-%s", lb.ID.String())
	// Cleanup if exists
	_ = a.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})

	cPort := nat.Port(fmt.Sprintf("%d/tcp", lb.Port))

	config_opt := &container.Config{
		Image: NginxImage,
		ExposedPorts: nat.PortSet{
			cPort: struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/nginx/nginx.conf:ro", filepath.Join(configPath, "nginx.conf")),
		},
		PortBindings: nat.PortMap{
			cPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: fmt.Sprintf("%d", lb.Port),
				},
			},
		},
	}

	// 5. Networking
	vpc, err := a.vpcRepo.GetByID(ctx, lb.VpcID)
	if err != nil {
		return "", err
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			vpc.NetworkID: {},
		},
	}

	resp, err := a.cli.ContainerCreate(ctx, config_opt, hostConfig, networkingConfig, nil, containerName)
	if err != nil {
		return "", err
	}

	if err := a.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (a *LBProxyAdapter) RemoveProxy(ctx context.Context, lbID uuid.UUID) error {
	containerName := fmt.Sprintf("lb-%s", lbID.String())
	err := a.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})
	if err != nil {
		log.Printf("Failed to remove container %s: %v", containerName, err)
	}

	// Cleanup config file
	configPath := filepath.Join("/tmp", "thecloud", "lb", lbID.String())
	_ = os.RemoveAll(configPath)

	return nil
}

func (a *LBProxyAdapter) UpdateProxyConfig(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) error {
	// Re-generate and overwrite config
	config, err := a.generateNginxConfig(ctx, lb, targets)
	if err != nil {
		return err
	}

	configPath := filepath.Join("/tmp", "thecloud", "lb", lb.ID.String(), "nginx.conf")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return err
	}

	containerName := fmt.Sprintf("lb-%s", lb.ID.String())
	// Try starting container if stopped
	_ = a.cli.ContainerStart(ctx, containerName, container.StartOptions{})

	// Reload nginx in container
	execResp, err := a.cli.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		Cmd: []string{"nginx", "-s", "reload"},
	})
	if err != nil {
		return err
	}

	return a.cli.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{})
}

func (a *LBProxyAdapter) generateNginxConfig(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) (string, error) {
	tmplRaw := `
user root;
events {
    worker_connections 1024;
}

http {
    {{if .Targets}}
    upstream backend {
        {{range .Targets}}
        server {{.ContainerID}}:{{.Port}} weight={{.Weight}};
        {{end}}
        {{if .LeastConn}}least_conn;{{end}}
    }
    {{end}}

    server {
        listen {{.Port}};
        location / {
            {{if .Targets}}
            proxy_pass http://backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            {{else}}
            return 503 "No targets available";
            {{end}}
        }
    }
}
`
	type targetInfo struct {
		ContainerID string
		Port        int
		Weight      int
	}
	type data struct {
		Port      int
		LeastConn bool
		Targets   []targetInfo
	}

	d := data{
		Port:      lb.Port,
		LeastConn: lb.Algorithm == "least-conn",
	}

	for _, t := range targets {
		inst, err := a.instanceRepo.GetByID(ctx, t.InstanceID)
		if err != nil {
			continue
		}
		// Predictable docker name used by InstanceService
		host := fmt.Sprintf("thecloud-%s", inst.ID.String()[:8])
		d.Targets = append(d.Targets, targetInfo{
			ContainerID: host,
			Port:        t.Port,
			Weight:      t.Weight,
		})
	}

	tmpl, err := template.New("nginx").Parse(tmplRaw)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return "", err
	}

	return buf.String(), nil
}
