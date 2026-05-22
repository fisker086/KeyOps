package alibaba

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/fisker086/keyops/pkg/resource"
)

func (p *Provider) createVPC(ctx context.Context, r *resource.Resource) (*resource.ResourceState, error) {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, fmt.Errorf("create vpc client: %w", err)
	}

	request := vpc.CreateCreateVpcRequest()
	request.Scheme = "https"

	if v, ok := r.Config["cidr_block"].(string); ok {
		request.CidrBlock = v
	}
	if v, ok := r.Config["vpc_name"].(string); ok {
		request.VpcName = v
	}
	if v, ok := r.Config["description"].(string); ok {
		request.Description = v
	}

	response, err := client.CreateVpc(request)
	if err != nil {
		return nil, fmt.Errorf("create vpc: %w", err)
	}

	props, _ := json.Marshal(map[string]interface{}{
		"vpc_id":    response.VpcId,
		"cidr_block": request.CidrBlock,
		"vpc_name":  request.VpcName,
		"region_id": p.region,
	})

	var properties map[string]interface{}
	json.Unmarshal(props, &properties)

	return &resource.ResourceState{
		Type:       "alicloud_vpc",
		Name:       r.Name,
		ResourceID: response.VpcId,
		Properties: properties,
		Status:     "created",
	}, nil
}

func (p *Provider) readVPC(ctx context.Context, state *resource.ResourceState) (*resource.ResourceState, error) {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, err
	}

	request := vpc.CreateDescribeVpcAttributeRequest()
	request.Scheme = "https"
	request.VpcId = state.ResourceID

	response, err := client.DescribeVpcAttribute(request)
	if err != nil {
		return nil, fmt.Errorf("describe vpc: %w", err)
	}

	props, _ := json.Marshal(map[string]interface{}{
		"vpc_id":    response.VpcId,
		"cidr_block": response.CidrBlock,
		"vpc_name":  response.VpcName,
		"status":    response.Status,
		"region_id": response.RegionId,
	})

	var properties map[string]interface{}
	json.Unmarshal(props, &properties)

	state.Properties = properties
	return state, nil
}

func (p *Provider) updateVPC(ctx context.Context, desired *resource.Resource, current *resource.ResourceState) (*resource.ResourceState, error) {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, err
	}

	request := vpc.CreateModifyVpcAttributeRequest()
	request.Scheme = "https"
	request.VpcId = current.ResourceID

	if v, ok := desired.Config["vpc_name"].(string); ok {
		request.VpcName = v
	}
	if v, ok := desired.Config["description"].(string); ok {
		request.Description = v
	}

	if _, err := client.ModifyVpcAttribute(request); err != nil {
		return nil, fmt.Errorf("modify vpc: %w", err)
	}

	return p.readVPC(ctx, &resource.ResourceState{
		Type:       "alicloud_vpc",
		ResourceID: current.ResourceID,
	})
}

func (p *Provider) deleteVPC(ctx context.Context, state *resource.ResourceState) error {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return err
	}

	request := vpc.CreateDeleteVpcRequest()
	request.Scheme = "https"
	request.VpcId = state.ResourceID

	_, err = client.DeleteVpc(request)
	return err
}
