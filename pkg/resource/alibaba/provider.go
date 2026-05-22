package alibaba

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/fisker086/keyops/pkg/resource"
)

type Provider struct {
	accessKey string
	secretKey string
	region    string
	client    *sdk.Client
}

func NewProvider(accessKey, secretKey, region string) (*Provider, error) {
	client, err := sdk.NewClientWithOptions(region,
		sdk.NewConfig(),
		credentials.NewAccessKeyCredential(accessKey, secretKey),
	)
	if err != nil {
		return nil, fmt.Errorf("create alicloud client: %w", err)
	}
	return &Provider{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		client:    client,
	}, nil
}

func (p *Provider) CloudType() string { return "alicloud" }

func (p *Provider) Create(ctx context.Context, r *resource.Resource) (*resource.ResourceState, error) {
	switch r.Type {
	case "alicloud_vpc":
		return p.createVPC(ctx, r)
	case "alicloud_vswitch":
		return p.createVSwitch(ctx, r)
	case "alicloud_security_group":
		return p.createSecurityGroup(ctx, r)
	default:
		return nil, fmt.Errorf("unsupported alicloud resource type: %s", r.Type)
	}
}

func (p *Provider) Read(ctx context.Context, state *resource.ResourceState) (*resource.ResourceState, error) {
	switch state.Type {
	case "alicloud_vpc":
		return p.readVPC(ctx, state)
	case "alicloud_vswitch":
		return p.readVSwitch(ctx, state)
	case "alicloud_security_group":
		return p.readSecurityGroup(ctx, state)
	default:
		return nil, fmt.Errorf("unsupported alicloud resource type: %s", state.Type)
	}
}

func (p *Provider) Update(ctx context.Context, desired *resource.Resource, current *resource.ResourceState) (*resource.ResourceState, error) {
	switch desired.Type {
	case "alicloud_vpc":
		return p.updateVPC(ctx, desired, current)
	case "alicloud_vswitch":
		return p.updateVSwitch(ctx, desired, current)
	case "alicloud_security_group":
		return p.updateSecurityGroup(ctx, desired, current)
	default:
		return nil, fmt.Errorf("unsupported alicloud resource type: %s", desired.Type)
	}
}

func (p *Provider) Delete(ctx context.Context, state *resource.ResourceState) error {
	switch state.Type {
	case "alicloud_vpc":
		return p.deleteVPC(ctx, state)
	case "alicloud_vswitch":
		return p.deleteVSwitch(ctx, state)
	case "alicloud_security_group":
		return p.deleteSecurityGroup(ctx, state)
	default:
		return fmt.Errorf("unsupported alicloud resource type: %s", state.Type)
	}
}

func (p *Provider) Diff(ctx context.Context, desired *resource.Resource, current *resource.ResourceState) (*resource.DiffResult, error) {
	return &resource.DiffResult{Action: "noop"}, nil
}
