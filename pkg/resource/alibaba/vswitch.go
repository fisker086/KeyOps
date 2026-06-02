package alibaba

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/vpc"
	"github.com/fisker086/keyops/pkg/resource"
)

func (p *Provider) createVSwitch(ctx context.Context, r *resource.Resource) (*resource.ResourceState, error) {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, fmt.Errorf("create vpc client: %w", err)
	}

	request := vpc.CreateCreateVSwitchRequest()
	request.Scheme = "https"

	if v, ok := r.Config["zone_id"].(string); ok {
		request.ZoneId = v
	}
	if v, ok := r.Config["cidr_block"].(string); ok {
		request.CidrBlock = v
	}
	if v, ok := r.Config["vpc_id"].(string); ok {
		request.VpcId = v
	}
	if v, ok := r.Config["vswitch_name"].(string); ok {
		request.VSwitchName = v
	}
	if v, ok := r.Config["description"].(string); ok {
		request.Description = v
	}

	response, err := client.CreateVSwitch(request)
	if err != nil {
		return nil, fmt.Errorf("create vswitch: %w", err)
	}

	props, _ := json.Marshal(map[string]interface{}{
		"vswitch_id":   response.VSwitchId,
		"vpc_id":       request.VpcId,
		"cidr_block":   request.CidrBlock,
		"zone_id":      request.ZoneId,
		"vswitch_name": request.VSwitchName,
	})

	var properties map[string]interface{}
	json.Unmarshal(props, &properties)

	return &resource.ResourceState{
		Type:       "alicloud_vswitch",
		Name:       r.Name,
		ResourceID: response.VSwitchId,
		Properties: properties,
		Status:     "created",
	}, nil
}

func (p *Provider) readVSwitch(ctx context.Context, state *resource.ResourceState) (*resource.ResourceState, error) {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, err
	}

	req := vpc.CreateDescribeVSwitchAttributesRequest()
	req.Scheme = "https"
	req.VSwitchId = state.ResourceID

	resp, err := client.DescribeVSwitchAttributes(req)
	if err != nil {
		return nil, fmt.Errorf("describe vswitch: %w", err)
	}

	props, _ := json.Marshal(map[string]interface{}{
		"vswitch_id":   resp.VSwitchId,
		"vpc_id":       resp.VpcId,
		"cidr_block":   resp.CidrBlock,
		"zone_id":      resp.ZoneId,
		"vswitch_name": resp.VSwitchName,
		"status":       resp.Status,
	})

	var properties map[string]interface{}
	json.Unmarshal(props, &properties)

	state.Properties = properties
	return state, nil
}

func (p *Provider) updateVSwitch(ctx context.Context, desired *resource.Resource, current *resource.ResourceState) (*resource.ResourceState, error) {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, err
	}

	req := vpc.CreateModifyVSwitchAttributeRequest()
	req.Scheme = "https"
	req.VSwitchId = current.ResourceID

	if v, ok := desired.Config["vswitch_name"].(string); ok {
		req.VSwitchName = v
	}
	if v, ok := desired.Config["description"].(string); ok {
		req.Description = v
	}

	if _, err := client.ModifyVSwitchAttribute(req); err != nil {
		return nil, fmt.Errorf("modify vswitch: %w", err)
	}

	return p.readVSwitch(ctx, &resource.ResourceState{
		Type:       "alicloud_vswitch",
		ResourceID: current.ResourceID,
	})
}

func (p *Provider) deleteVSwitch(ctx context.Context, state *resource.ResourceState) error {
	client, err := vpc.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return err
	}

	request := vpc.CreateDeleteVSwitchRequest()
	request.Scheme = "https"
	request.VSwitchId = state.ResourceID

	_, err = client.DeleteVSwitch(request)
	return err
}
